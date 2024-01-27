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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
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

	namespace := bp.GetNamespace()
	name := bp.GetName()

	if condition.IsBackupInvalid(bp) {
		klog.Infof("Skipping sync because NebulaBackup [%s/%s] is invalid.", namespace, name)
		return nil
	}

	if condition.IsBackupComplete(bp) {
		klog.Infof("Skipping sync because NebulaBackup [%s/%s] is already complete.", namespace, name)
		return nil
	}

	if condition.IsBackupFailed(bp) {
		klog.Infof("Skipping sync because NebulaBackup [%s/%s] has already failed.", namespace, name)
		return nil
	}

	err := c.backupManager.Sync(bp)
	if err != nil && !errors.IsReconcileError(err) {
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err := c.clientSet.NebulaBackup().UpdateNebulaBackupStatus(bp, &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupFailed,
			Status: corev1.ConditionTrue,
			Reason: err.Error(),
		}, &kube.BackupUpdateStatus{
			ConditionType: v1alpha1.BackupFailed,
		}); err != nil {
			return fmt.Errorf("failed to update NebulaBackup [%s/%s], %v", namespace, name, err)
		}

		return nil
	}

	return err
}
