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

package nebulabackup

import (
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

type ControlInterface interface {
	SyncNebulaBackup(bp *v1alpha1.NebulaBackup) error
}

var _ ControlInterface = (*defaultBackupControl)(nil)

type defaultBackupControl struct {
	clientSet     kube.ClientSet
	backupManager Manager
}

func NewBackupControl(clientSet kube.ClientSet, backupManager Manager) ControlInterface {
	return &defaultBackupControl{
		clientSet:     clientSet,
		backupManager: backupManager,
	}
}

func (c *defaultBackupControl) SyncNebulaBackup(bp *v1alpha1.NebulaBackup) error {
	phase := bp.Status.Phase
	if phase == "" {
		phase = v1alpha1.BackupPending
	}

	switch phase {
	case v1alpha1.BackupPending:
		klog.Infof("create backup job %s", bp.Name)
		err := c.backupManager.Create(bp)
		if err != nil && !errors.IsReconcileError(err) {
			klog.Errorf("Fail to create NebulaBackup [%s/%s], %v", bp.Namespace, bp.Name, err)
			if err = c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(bp, &v1alpha1.BackupCondition{
				Type:    v1alpha1.BackupFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "CreateBackupJobFailed",
				Message: err.Error(),
			}, &kube.BackupUpdateStatus{
				ConditionType: v1alpha1.BackupFailed,
			}); err != nil {
				klog.Errorf("Fail to update the condition of NebulaBackup [%s/%s], %v", bp.Namespace, bp.Name, err)
			}
		}
		return err
	case v1alpha1.BackupRunning:
		klog.Infof("sync backup job %s", bp.Name)
		err := c.backupManager.Sync(bp)
		if err != nil && !errors.IsReconcileError(err) {
			klog.Errorf("Fail to sync NebulaBackup [%s/%s], %v", bp.Namespace, bp.Name, err)
			if err = c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(bp, &v1alpha1.BackupCondition{
				Type:    v1alpha1.BackupFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "SyncNebulaBackupFailed",
				Message: err.Error(),
			}, &kube.BackupUpdateStatus{
				ConditionType: v1alpha1.BackupFailed,
			}); err != nil {
				klog.Errorf("Fail to update the condition of NebulaBackup [%s/%s], %v", bp.Namespace, bp.Name, err)
			}
		}
		return err
	}
	klog.Infof("sync NebulaBackup success, backup %s phase is %s", bp.Name, phase)

	return nil
}
