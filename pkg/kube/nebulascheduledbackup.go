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

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type ScheduledBackupUpdateStatus struct {
	// Used for scheduled incremental backups. Not supported for now.
	// LastBackup     string
	CurrPauseStatus          *bool
	LastScheduledBackupTime  *metav1.Time
	LastSuccessfulBackupTime *metav1.Time
	MostRecentJobFailed      *bool
}

type NebulaScheduledBackup interface {
	GetNebulaScheduledBackup(namespace, name string) (*v1alpha1.NebulaScheduledBackup, error)
	SetNebulaScheduledBackupStatus(backup *v1alpha1.NebulaScheduledBackup, newStatus *ScheduledBackupUpdateStatus) error
}

type scheduledBackupClient struct {
	cli client.Client
}

func NewScheduledNebulaBackup(cli client.Client) NebulaScheduledBackup {
	return &scheduledBackupClient{cli: cli}
}

func (r *scheduledBackupClient) GetNebulaScheduledBackup(namespace, name string) (*v1alpha1.NebulaScheduledBackup, error) {
	scheduledBackup := &v1alpha1.NebulaScheduledBackup{}
	err := r.cli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, scheduledBackup)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaScheduledBackup", "namespace", namespace, "name", name)
		return nil, err
	}
	return scheduledBackup, nil
}

func (r *scheduledBackupClient) SetNebulaScheduledBackupStatus(backup *v1alpha1.NebulaScheduledBackup, newStatus *ScheduledBackupUpdateStatus) error {
	var isStatusUpdate bool
	ns := backup.GetNamespace()
	rtName := backup.GetName()

	return retry.OnError(retry.DefaultRetry, func(e error) bool { return e != nil }, func() error {
		if updated, err := r.GetNebulaScheduledBackup(ns, rtName); err == nil {
			backup = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaScheduledBackup [%s/%s] failed: %v", ns, rtName, err))
			return err
		}

		// Make sure current resource version changes to avoid immediate reconcile if no error
		updateErr := r.cli.Update(context.TODO(), backup)
		if updateErr != nil {
			klog.Errorf("update NebulaScheduledBackup [%s/%s] status failed: %v", ns, rtName, updateErr)
			return updateErr
		}

		isStatusUpdate = updateScheduledBackupStatus(&backup.Status, newStatus)
		if isStatusUpdate {
			updateErr = r.cli.Status().Update(context.TODO(), backup)
			if updateErr == nil {
				klog.Infof("NebulaScheduledBackup [%s/%s] updated successfully", ns, rtName)
				return nil
			}
			klog.Errorf("update NebulaScheduledBackup [%s/%s] status failed: %v", ns, rtName, updateErr)
			return updateErr
		}
		return nil
	})
}

func updateScheduledBackupStatus(status *v1alpha1.ScheduledBackupStatus, newStatus *ScheduledBackupUpdateStatus) bool {
	if newStatus == nil {
		return false
	}

	isUpdate := false
	// Used for scheduled incremental backups. Not supported for now.
	/* if newStatus.LastBackup != "" {
		status.LastBackup = newStatus.LastBackup
		isUpdate = true
	} */
	if newStatus.CurrPauseStatus != status.CurrPauseStatus {
		status.CurrPauseStatus = newStatus.CurrPauseStatus
		isUpdate = true
	}

	if newStatus.LastScheduledBackupTime != nil {
		status.LastScheduledBackupTime = newStatus.LastScheduledBackupTime
		isUpdate = true
	}

	if newStatus.LastSuccessfulBackupTime != nil {
		status.LastSuccessfulBackupTime = newStatus.LastSuccessfulBackupTime
		isUpdate = true
	}

	if newStatus.MostRecentJobFailed != nil {
		status.MostRecentJobFailed = newStatus.MostRecentJobFailed
		isUpdate = true
	}

	return isUpdate
}