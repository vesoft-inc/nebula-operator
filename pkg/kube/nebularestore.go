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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	condutil "github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

type RestoreUpdateStatus struct {
	TimeStarted     *metav1.Time
	TimeCompleted   *metav1.Time
	ClusterName     *string
	ConditionType   v1alpha1.RestoreConditionType
	Partitions      map[string][]*nebula.HostAddr
	Checkpoints     map[string]map[string]string
	MetaDownload    bool
	StorageDownload bool
}

type NebulaRestore interface {
	GetNebulaRestore(namespace, name string) (*v1alpha1.NebulaRestore, error)
	UpdateNebulaRestoreStatus(restore *v1alpha1.NebulaRestore, condition *v1alpha1.RestoreCondition, newStatus *RestoreUpdateStatus) error
}

type restoreClient struct {
	client client.Client
}

func NewNebulaRestore(client client.Client) NebulaRestore {
	return &restoreClient{client: client}
}

func (r *restoreClient) GetNebulaRestore(namespace, name string) (*v1alpha1.NebulaRestore, error) {
	restore := &v1alpha1.NebulaRestore{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, restore)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaRestore", "namespace", namespace, "name", name)
		return nil, err
	}
	return restore, nil
}

func (r *restoreClient) UpdateNebulaRestoreStatus(restore *v1alpha1.NebulaRestore, condition *v1alpha1.RestoreCondition, newStatus *RestoreUpdateStatus) error {
	var isStatusUpdate bool
	var isConditionUpdate bool
	ns := restore.GetNamespace()
	rtName := restore.GetName()

	return retry.OnError(retry.DefaultRetry, func(e error) bool { return e != nil }, func() error {
		if updated, err := r.GetNebulaRestore(ns, rtName); err == nil {
			restore = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaRestore [%s/%s] failed: %v", ns, rtName, err))
			return err
		}
		isStatusUpdate = updateRestoreStatus(&restore.Status, newStatus)
		isConditionUpdate = condutil.UpdateNebulaRestoreCondition(&restore.Status, condition)
		if isStatusUpdate || isConditionUpdate {
			updateErr := r.client.Status().Update(context.TODO(), restore)
			if updateErr == nil {
				klog.Infof("NebulaRestore [%s/%s] updated successfully", ns, rtName)
				return nil
			}
			klog.Errorf("update NebulaRestore [%s/%s] status failed: %v", ns, rtName, updateErr)
			return updateErr
		}
		return nil
	})
}

func updateRestoreStatus(status *v1alpha1.RestoreStatus, newStatus *RestoreUpdateStatus) bool {
	if newStatus == nil {
		return false
	}

	isUpdate := false
	if status.Phase != newStatus.ConditionType {
		status.Phase = newStatus.ConditionType
		isUpdate = true
	}
	if newStatus.ClusterName != nil {
		status.ClusterName = *newStatus.ClusterName
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
	if len(status.Partitions) != len(newStatus.Partitions) {
		status.Partitions = newStatus.Partitions
		isUpdate = true
	}
	if len(status.Checkpoints) != len(newStatus.Checkpoints) {
		status.Checkpoints = newStatus.Checkpoints
		isUpdate = true
	}
	if newStatus.MetaDownload {
		status.MetaDownload = newStatus.MetaDownload
		isUpdate = true
	}
	if newStatus.StorageDownload {
		status.StorageDownload = newStatus.StorageDownload
		isUpdate = true
	}

	return isUpdate
}
