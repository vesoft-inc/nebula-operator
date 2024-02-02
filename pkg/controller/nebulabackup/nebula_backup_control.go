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

package nebulabackup

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	finalizer = "apps.nebula-graph.io/backup-cleanup"
)

type ControlInterface interface {
	UpdateNebulaBackup(backup *v1alpha1.NebulaBackup) error
}

var _ ControlInterface = (*defaultBackupControl)(nil)

type defaultBackupControl struct {
	client        client.Client
	clientSet     kube.ClientSet
	backupManager Manager
}

func NewBackupControl(client client.Client, clientSet kube.ClientSet, backupManager Manager) ControlInterface {
	return &defaultBackupControl{
		client:        client,
		clientSet:     clientSet,
		backupManager: backupManager,
	}
}

func (c *defaultBackupControl) UpdateNebulaBackup(backup *v1alpha1.NebulaBackup) error {
	isDeleting := !backup.GetDeletionTimestamp().IsZero()
	if !isDeleting {
		if err := c.addFinalizer(backup); err != nil {
			return err
		}
	}

	if isDeleting {
		if err := c.backupManager.Clean(backup); err != nil {
			if !utilerrors.IsReconcileError(err) {
				if uErr := c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
					Type:    v1alpha1.BackupFailed,
					Status:  corev1.ConditionTrue,
					Reason:  "CleanFailed",
					Message: err.Error(),
				}, &kube.BackupUpdateStatus{
					ConditionType: v1alpha1.BackupFailed,
				}); uErr != nil {
					klog.Errorf("failed to update the condition of backup [%s/%s], %v", backup.Namespace, backup.Name, uErr)
				}
				if pointer.BoolDeref(backup.Spec.AutoRemoveFinished, false) {
					cleanJobName := getCleanJobName(backup.Name)
					if err := c.clientSet.Job().DeleteJob(backup.Namespace, cleanJobName); err != nil {
						klog.Errorf("failed to delete job [%s/%s], %v", backup.Namespace, cleanJobName, err)
					}
				}
				return nil
			}
			return err
		}
		return c.removeFinalizer(backup)
	}

	if condition.IsBackupInvalid(backup) {
		klog.Infof("skipping because backup [%s/%s] is invalid.", backup.Namespace, backup.Name)
		return nil
	}

	if condition.IsBackupComplete(backup) {
		klog.Infof("skipping because backup [%s/%s] is complete.", backup.Namespace, backup.Name)
		return nil
	}

	if condition.IsBackupFailed(backup) {
		klog.Infof("skipping because backup [%s/%s] is failed.", backup.Namespace, backup.Name)
		return nil
	}

	if err := c.backupManager.Sync(backup); err != nil {
		if !utilerrors.IsReconcileError(err) {
			if uErr := c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
				Type:    v1alpha1.BackupFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "BackupFailed",
				Message: err.Error(),
			}, &kube.BackupUpdateStatus{
				ConditionType: v1alpha1.BackupFailed,
			}); uErr != nil {
				klog.Errorf("failed to update the condition of backup [%s/%s], %v", backup.Namespace, backup.Name, uErr)
			}
			if pointer.BoolDeref(backup.Spec.AutoRemoveFinished, false) {
				backupType := getBackupType(backup.Spec.Config.BaseBackupName)
				backupJobName := getBackupJobName(backup.Name, backupType.String())
				if err := c.clientSet.Job().DeleteJob(backup.Namespace, backupJobName); err != nil {
					klog.Errorf("failed to delete job [%s/%s], %v", backup.Namespace, backupJobName, err)
				}
			}
			return nil
		}
		return err
	}

	return nil
}

func (c *defaultBackupControl) addFinalizer(backup *v1alpha1.NebulaBackup) error {
	if needToAddFinalizer(backup) {
		if err := kube.UpdateFinalizer(context.TODO(), c.client, backup, kube.AddFinalizerOpType, finalizer); err != nil {
			return fmt.Errorf("add backup [%s/%s] finalizer failed, err: %v", backup.Namespace, backup.Name, err)
		}
	}
	return nil
}

func (c *defaultBackupControl) removeFinalizer(backup *v1alpha1.NebulaBackup) error {
	if needToRemoveFinalizer(backup) {
		if err := kube.UpdateFinalizer(context.TODO(), c.client, backup, kube.RemoveFinalizerOpType, finalizer); err != nil {
			return fmt.Errorf("remove backup [%s/%s] finalizer failed, err: %v", backup.Namespace, backup.Name, err)
		}
	}
	return nil
}

func needToAddFinalizer(backup *v1alpha1.NebulaBackup) bool {
	return backup.CleanBackupData() &&
		!kube.HasFinalizer(backup, finalizer)
}

func needToRemoveFinalizer(backup *v1alpha1.NebulaBackup) bool {
	return backup.CleanBackupData() &&
		kube.HasFinalizer(backup, finalizer) &&
		condition.IsBackupClean(backup)
}
