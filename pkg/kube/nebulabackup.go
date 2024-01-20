/*
Copyright 2023 Vesoft Inc.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	condutil "github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

type BackupUpdateStatus struct {
	TimeStarted   *metav1.Time
	TimeCompleted *metav1.Time
	ConditionType v1alpha1.BackupConditionType
}

type NebulaBackup interface {
	CreateNebulaBackup(nb *v1alpha1.NebulaBackup) error
	GetNebulaBackup(namespace, name string) (*v1alpha1.NebulaBackup, error)
	UpdateNebulaBackupStatus(backup *v1alpha1.NebulaBackup, condition *v1alpha1.BackupCondition, newStatus *BackupUpdateStatus) error
	DeleteNebulaBackup(namespace, name string) error
	ListNebulaBackupsByUID(namespace string, ownerReferenceUID types.UID) ([]v1alpha1.NebulaBackup, error)
}

type backupClient struct {
	cli client.Client
}

func NewNebulaBackup(cli client.Client) NebulaBackup {
	return &backupClient{cli: cli}
}

func (c *backupClient) CreateNebulaBackup(nb *v1alpha1.NebulaBackup) error {
	if err := c.cli.Create(context.TODO(), nb); err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("NebulaBackup %s/%s already exists", nb.Namespace, nb.Name)
			return nil
		}
		return err
	}
	return nil
}

func (r *backupClient) GetNebulaBackup(namespace, name string) (*v1alpha1.NebulaBackup, error) {
	backup := &v1alpha1.NebulaBackup{}
	err := r.cli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, backup)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaBackup", "namespace", namespace, "name", name)
		return nil, err
	}
	return backup, nil
}

func (r *backupClient) ListNebulaBackupsByUID(namespace string, ownerReferenceUID types.UID) ([]v1alpha1.NebulaBackup, error) {
	backupList := v1alpha1.NebulaBackupList{}
	if err := r.cli.List(context.TODO(), &backupList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var filteredBackups []v1alpha1.NebulaBackup
	for _, backup := range backupList.Items {
		for _, ownerRef := range backup.OwnerReferences {
			if ownerRef.UID == ownerReferenceUID {
				filteredBackups = append(filteredBackups, backup)
			}
		}
	}

	return filteredBackups, nil
}

func (r *backupClient) UpdateNebulaBackupStatus(backup *v1alpha1.NebulaBackup, condition *v1alpha1.BackupCondition, newStatus *BackupUpdateStatus) error {
	var isStatusUpdate bool
	var isConditionUpdate bool
	ns := backup.GetNamespace()
	rtName := backup.GetName()

	return retry.OnError(retry.DefaultRetry, func(e error) bool { return e != nil }, func() error {
		if updated, err := r.GetNebulaBackup(ns, rtName); err == nil {
			backup = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaBackup [%s/%s] failed: %v", ns, rtName, err))
			return err
		}

		// Make sure current resource version changes to avoid immediate reconcile if no error
		updateErr := r.cli.Update(context.TODO(), backup)
		if updateErr != nil {
			klog.Errorf("update NebulaScheduledBackup [%s/%s] status failed: %v", ns, rtName, updateErr)
			return updateErr
		}

		isStatusUpdate = updateBackupStatus(&backup.Status, newStatus)
		isConditionUpdate = condutil.UpdateNebulaBackupCondition(&backup.Status, condition)
		if isStatusUpdate || isConditionUpdate {
			updateErr := r.cli.Status().Update(context.TODO(), backup)
			if updateErr == nil {
				klog.Infof("NebulaBackup [%s/%s] updated successfully", ns, rtName)
				return nil
			}
			klog.Errorf("update NebulaBackup [%s/%s] status failed: %v", ns, rtName, updateErr)
			return updateErr
		}
		return nil
	})
}

func (r *backupClient) DeleteNebulaBackup(namespace, name string) error {
	nb, err := r.GetNebulaBackup(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.cli.Delete(context.TODO(), nb); err != nil {
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
	if status.Phase != newStatus.ConditionType {
		status.Phase = newStatus.ConditionType
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

	return isUpdate
}