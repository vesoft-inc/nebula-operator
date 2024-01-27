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
	"time"

	"k8s.io/klog/v2"

	condutil "github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

type ControlInterface interface {
	Sync(bp *v1alpha1.NebulaScheduledBackup) (*time.Duration, error)
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

func (c *defaultScheduledBackupControl) Sync(sbp *v1alpha1.NebulaScheduledBackup) (*time.Duration, error) {
	ownedNebulaBackups, err := c.clientSet.NebulaBackup().ListNebulaBackupsByUID(sbp.Namespace, sbp.UID)
	if err != nil {
		klog.Errorf("Fail to get nebula backup jobs owned by [%s/%s], err: %v", sbp.Namespace, sbp.Name, err)
		return nil, err
	}

	var successfulBackups, failedBackups, runningBackups []v1alpha1.NebulaBackup
	for _, nebulaBackup := range ownedNebulaBackups {
		if condutil.IsBackupComplete(&nebulaBackup) {
			successfulBackups = append(successfulBackups, nebulaBackup)
		} else if condutil.IsBackupFailed(&nebulaBackup) && condutil.IsBackupInvalid(&nebulaBackup) {
			failedBackups = append(failedBackups, nebulaBackup)
		} else {
			runningBackups = append(runningBackups, nebulaBackup)
		}
	}

	err = c.scheduledBackupManager.CleanupFinishedNBs(sbp, successfulBackups, failedBackups)
	if err != nil {
		klog.Errorf("Fail to cleanup nebula backup jobs owned by [%s/%s], err: %v", sbp.Namespace, sbp.Name, err)
		return nil, err
	}

	now := metav1.Now()

	syncUpdates, NextBackupTime, err := c.scheduledBackupManager.SyncNebulaScheduledBackup(sbp, successfulBackups, failedBackups, runningBackups, &now)
	if err != nil {
		return nil, utilerrors.ReconcileErrorf("error syncing Nebula scheduled backup [%s/%s]. Err: %v", sbp.Namespace, sbp.Name, err)
	}

	if syncUpdates != nil {
		klog.Infof("Updating Nebula scheduled backup [%s/%s]", sbp.Namespace, sbp.Name)
		err = c.clientSet.NebulaScheduledBackup().SetNebulaScheduledBackupStatus(sbp, syncUpdates)
		if err != nil {
			klog.Errorf("Fail to update Nebula scheduled backup [%s/%s], err: %v", sbp.Namespace, sbp.Name, err)
			return nil, err
		}
		klog.Infof("Nebula scheduled backup [%s/%s] updated successfully.", sbp.Namespace, sbp.Name)
	} else {
		klog.Infof("No status updates needed for Nebula scheduled backup [%s/%s]", sbp.Namespace, sbp.Name)
	}

	klog.Infof("Reconcile NebulaScheduledBackup success for Nebula scheduled backup %s", sbp.Name)

	if NextBackupTime != nil {
		newReconcilDuration := NextBackupTime.Sub(now.Local())
		return &newReconcilDuration, nil
	}

	return nil, nil
}
