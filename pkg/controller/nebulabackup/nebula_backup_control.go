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
	"fmt"
	"time"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"k8s.io/klog/v2"
)

type ControlInterface interface {
	SyncNebulaBackup(bp *v1alpha1.NebulaBackup) (*time.Duration, error)
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

func (c *defaultBackupControl) SyncNebulaBackup(bp *v1alpha1.NebulaBackup) (*time.Duration, error) {

	updateCondition, updateStatus, syncErr := c.backupManager.Sync(bp)

	if syncErr != nil {
		klog.Errorf("Fail to sync NebulaBackup [%s/%s], %v", bp.Namespace, bp.Name, syncErr)
	}

	if updateCondition != nil && updateStatus != nil {
		klog.Infof("Update status for NebulaBackup [%s/%s]", bp.Namespace, bp.Name)
		if updateErr := c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(bp, updateCondition, updateStatus); updateErr != nil {
			return nil, fmt.Errorf("update nebula backup %s/%s status err: %w", bp.Namespace, bp.Name, updateErr)
		}
		klog.Infof("Successfully updated status for NebulaBackup [%s/%s]", bp.Namespace, bp.Name)

		if updateStatus.ConditionType == v1alpha1.BackupFailed || updateStatus.ConditionType == v1alpha1.BackupInvalid {
			return nil, syncErr
		}

		if updateStatus.ConditionType == v1alpha1.BackupComplete {
			return nil, nil
		}

	} else {
		if bp.Status.Phase == v1alpha1.BackupComplete {
			return nil, nil
		}
		if bp.Status.Phase == v1alpha1.BackupFailed || bp.Status.Phase == v1alpha1.BackupInvalid {
			return nil, utilerrors.ReconcileErrorf("error syncing Nebula backup [%s/%s] for a second time.", bp.Namespace, bp.Name)
		}
	}

	klog.Infof("Waiting for NebulaBackup [%s/%s] to complete", bp.Namespace, bp.Name)
	nextReconcile := 15 * time.Second
	return &nextReconcile, nil
}
