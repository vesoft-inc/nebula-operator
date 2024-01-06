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

package nebulascheduledbackup

import (
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

type ControlInterface interface {
	SyncNebulaScheduledBackup(bp *v1alpha1.NebulaScheduledBackup) error
}

var _ ControlInterface = (*defaultScheduledBackupControl)(nil)

type defaultScheduledBackupControl struct {
	clientSet              kube.ClientSet
	scheduledBackupManager Manager
}

func NewBackupControl(clientSet kube.ClientSet, scheduledBackupManager Manager) ControlInterface {
	return &defaultScheduledBackupControl{
		clientSet:              clientSet,
		scheduledBackupManager: scheduledBackupManager,
	}
}

func (c *defaultScheduledBackupControl) SyncNebulaScheduledBackup(sbp *v1alpha1.NebulaScheduledBackup) error {
	phase := sbp.Status.Phase
	if phase == "" {
		phase = v1alpha1.ScheduledBackupPending
	}

	switch phase {
	case v1alpha1.ScheduledBackupPending:
		klog.Infof("creating scheduled backup job %s", sbp.Name)
		err := c.scheduledBackupManager.Create(sbp)
		if err != nil && !errors.IsReconcileError(err) {
			klog.Errorf("Fail to create NebulaScheduledBackup [%s/%s], %v", sbp.Namespace, sbp.Name, err)
			if err = c.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(sbp, &kube.ScheduledBackupUpdateStatus{
				Phase: v1alpha1.ScheduledBackupFailed,
			}); err != nil {
				klog.Errorf("Fail to update the condition of NebulaScheduledBackup [%s/%s], %v", sbp.Namespace, sbp.Name, err)
			}
		}
		return err
	case v1alpha1.ScheduledBackupPaused:
		if !sbp.Spec.Pause {
			klog.Infof("resume scheduled backup job %s", sbp.Name)
			err := c.scheduledBackupManager.Resume(sbp)
			if err != nil && !errors.IsReconcileError(err) {
				klog.Errorf("Fail to resume NebulaScheduledBackup [%s/%s], %v", sbp.Namespace, sbp.Name, err)
				if err = c.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(sbp, &kube.ScheduledBackupUpdateStatus{
					Phase: v1alpha1.ScheduledBackupFailed,
				}); err != nil {
					klog.Errorf("Fail to update the condition of NebulaScheduledBackup [%s/%s], %v", sbp.Namespace, sbp.Name, err)
				}
			}
		}
	case v1alpha1.ScheduledBackupFailed:
		klog.Infof("skipping reconciling NebulaScheduledBackup [%s/%s]. It is in %s phase", sbp.Namespace, sbp.Name, v1alpha1.ScheduledBackupFailed)
		return nil
	default:
		var err error
		var transaction string
		if sbp.Spec.Pause {
			transaction = "pause"
			klog.Infof("pause scheduled backup job %s", sbp.Name)
			err = c.scheduledBackupManager.Pause(sbp)
		} else {
			transaction = "sync"
			klog.Infof("sync scheduled backup job %s", sbp.Name)
			err = c.scheduledBackupManager.Sync(sbp)
		}
		if err != nil && !errors.IsReconcileError(err) {
			klog.Errorf("Fail to %v NebulaScheduledBackup [%s/%s], %v", transaction, sbp.Namespace, sbp.Name, err)
			if err = c.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(sbp, &kube.ScheduledBackupUpdateStatus{
				Phase: v1alpha1.ScheduledBackupFailed,
			}); err != nil {
				klog.Errorf("Fail to update the condition of NebulaScheduledBackup [%s/%s], %v", sbp.Namespace, sbp.Name, err)
			}
		}
		return err
	}

	klog.Infof("Reconcile NebulaScheduledBackup success, scheduled backup %s phase is %s", sbp.Name, phase)
	return nil
}
