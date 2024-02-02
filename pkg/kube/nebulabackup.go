/*
Copyright 2024 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	condutil "github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

type BackupUpdateStatus struct {
	Type          v1alpha1.BackupType
	BackupName    *string
	TimeStarted   *metav1.Time
	TimeCompleted *metav1.Time
	ConditionType v1alpha1.BackupConditionType
}

type NebulaBackup interface {
	CreateNebulaBackup(backup *v1alpha1.NebulaBackup) error
	GetNebulaBackup(namespace, name string) (*v1alpha1.NebulaBackup, error)
	ListNebulaBackups(namespace string, selector labels.Selector) ([]v1alpha1.NebulaBackup, error)
	UpdateNebulaBackup(backup *v1alpha1.NebulaBackup) error
	UpdateNebulaBackupStatus(backup *v1alpha1.NebulaBackup, condition *v1alpha1.BackupCondition, newStatus *BackupUpdateStatus) error
	DeleteNebulaBackup(namespace, name string) error
}

type backupClient struct {
	client client.Client
}

func NewNebulaBackup(client client.Client) NebulaBackup {
	return &backupClient{client: client}
}

func (b *backupClient) CreateNebulaBackup(nb *v1alpha1.NebulaBackup) error {
	if err := b.client.Create(context.TODO(), nb); err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("NebulaBackup %s/%s already exists", nb.Namespace, nb.Name)
			return nil
		}
		return err
	}
	return nil
}

func (b *backupClient) GetNebulaBackup(namespace, name string) (*v1alpha1.NebulaBackup, error) {
	backup := &v1alpha1.NebulaBackup{}
	err := b.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, backup)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaBackup", "namespace", namespace, "name", name)
		return nil, err
	}
	return backup, nil
}

func (b *backupClient) ListNebulaBackups(namespace string, selector labels.Selector) ([]v1alpha1.NebulaBackup, error) {
	backupList := &v1alpha1.NebulaBackupList{}
	if err := b.client.List(context.TODO(), backupList, &client.ListOptions{LabelSelector: selector, Namespace: namespace}); err != nil {
		return nil, err
	}

	return backupList.Items, nil
}

func (b *backupClient) UpdateNebulaBackup(backup *v1alpha1.NebulaBackup) error {
	ns := backup.Namespace
	backupName := backup.Name
	backupSpec := backup.Spec.DeepCopy()
	backupLabels := backup.GetLabels()
	annotations := backup.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Update the set with the latest resource version for the next poll
		backupClone, err := b.GetNebulaBackup(ns, backupName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("get NebulaBackup %s/%s failed: %v", ns, backupName, err))
			return err
		}

		if reflect.DeepEqual(backupSpec, backupClone.Spec) &&
			reflect.DeepEqual(backupLabels, backupClone.Labels) &&
			reflect.DeepEqual(annotations, backupClone.Annotations) {
			return nil
		}

		backup = backupClone.DeepCopy()
		backup.Spec = *backupSpec
		backup.SetLabels(backupLabels)
		backup.SetAnnotations(annotations)
		updateErr := b.client.Update(context.TODO(), backup)
		if updateErr == nil {
			klog.Infof("NebulaBackup %s/%s updated successfully", ns, backupName)
			return nil
		}
		klog.Errorf("update NebulaBackup [%s/%s] failed: %v", ns, backupName, updateErr)
		return updateErr
	})
}

func (b *backupClient) UpdateNebulaBackupStatus(backup *v1alpha1.NebulaBackup, condition *v1alpha1.BackupCondition, newStatus *BackupUpdateStatus) error {
	var isStatusUpdate bool
	var isConditionUpdate bool
	ns := backup.GetNamespace()
	backupName := backup.GetName()

	return retry.OnError(retry.DefaultRetry, func(e error) bool { return e != nil }, func() error {
		if updated, err := b.GetNebulaBackup(ns, backupName); err == nil {
			backup = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaBackup [%s/%s] failed: %v", ns, backupName, err))
			return err
		}
		isStatusUpdate = updateBackupStatus(&backup.Status, newStatus)
		isConditionUpdate = condutil.UpdateNebulaBackupCondition(&backup.Status, condition)
		if isStatusUpdate || isConditionUpdate {
			updateErr := b.client.Status().Update(context.TODO(), backup)
			if updateErr == nil {
				klog.Infof("NebulaBackup [%s/%s] updated successfully", ns, backupName)
				return nil
			}
			klog.Errorf("update NebulaBackup [%s/%s] status failed: %v", ns, backupName, updateErr)
			return updateErr
		}
		return nil
	})
}

func (b *backupClient) DeleteNebulaBackup(namespace, name string) error {
	backup, err := b.GetNebulaBackup(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := b.client.Delete(context.TODO(), backup); err != nil {
		return err
	}
	klog.Infof("NebulaBackup [%s/%s] deleted successfully", namespace, name)
	return nil
}

func updateBackupStatus(status *v1alpha1.BackupStatus, newStatus *BackupUpdateStatus) bool {
	if newStatus == nil {
		return false
	}

	isUpdate := false
	if newStatus.Type != "" {
		status.Type = newStatus.Type
		isUpdate = true
	}
	if newStatus.BackupName != nil {
		status.BackupName = *newStatus.BackupName
		isUpdate = true
	}
	if newStatus.TimeStarted != nil {
		status.TimeStarted = newStatus.TimeStarted
		isUpdate = true
	}
	if newStatus.TimeCompleted != nil {
		status.TimeCompleted = newStatus.TimeCompleted
		isUpdate = true
	}
	if status.Phase != newStatus.ConditionType {
		status.Phase = newStatus.ConditionType
		isUpdate = true
	}

	return isUpdate
}
