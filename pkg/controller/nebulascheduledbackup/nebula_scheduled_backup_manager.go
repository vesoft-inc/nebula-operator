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
	"fmt"

	"github.com/robfig/cron"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebulabackup"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	condutil "github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	EnvS3AccessKeyName = "S3_ACCESS_KEY"
	EnvS3SecretKeyName = "S3_SECRET_KEY"
	BackupPrefix       = "nsb"
)

type Manager interface {
	// Create creates a scheduled backup job.
	Create(backup *v1alpha1.NebulaScheduledBackup) error

	// Sync	implements the logic for syncing NebulaScheduledBackup.
	Sync(backup *v1alpha1.NebulaScheduledBackup) error

	// Pause implements the logic for pauseing a NebulaScheduledBackup.
	Pause(backup *v1alpha1.NebulaScheduledBackup) error

	// Resume implements the logic for resuming a NebulaScheduledBackup.
	Resume(backup *v1alpha1.NebulaScheduledBackup) error
}

var _ Manager = (*scheduledBackupManager)(nil)

type scheduledBackupManager struct {
	clientSet kube.ClientSet
}

func NewBackupManager(clientSet kube.ClientSet) Manager {
	return &scheduledBackupManager{clientSet: clientSet}
}

func (bm *scheduledBackupManager) Create(scheduledBackup *v1alpha1.NebulaScheduledBackup) error {
	ns := scheduledBackup.GetNamespace()
	if scheduledBackup.Spec.BackupTemplate.BR.ClusterNamespace != nil {
		ns = *scheduledBackup.Spec.BackupTemplate.BR.ClusterNamespace
	}

	nc, err := bm.clientSet.NebulaCluster().GetNebulaCluster(ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName)
	if err != nil {
		return fmt.Errorf("get nebula cluster %s/%s err: %w", ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName, err)
	}

	if !nc.IsReady() {
		return fmt.Errorf("nebula cluster %s/%s is not ready", ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName)
	}

	nextRunTime, err := calculateNextRunTime(scheduledBackup)
	if err != nil {
		return fmt.Errorf("calculating next runtime for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}
	if nextRunTime.IsZero() {
		return fmt.Errorf("invalid schedule \"%v\" for scheduled backup %s/%s", scheduledBackup.Spec.Schedule, scheduledBackup.Namespace, scheduledBackup.Name)
	}

	if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
		// Used for scheduled incremental backups. Not supported for now.
		// LastBackup:     "N/A",
		CurrSchedule:   scheduledBackup.Spec.Schedule,
		LastBackupTime: &metav1.Time{},
		NextBackupTime: nextRunTime,
		Phase:          v1alpha1.ScheduledBackupScheduled,
	}); err != nil {
		return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}
	return nil
}

func (bm *scheduledBackupManager) Pause(scheduledBackup *v1alpha1.NebulaScheduledBackup) error {
	if err := bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
		Phase: v1alpha1.ScheduledBackupPaused,
	}); err != nil {
		return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}
	return utilerrors.ReconcileErrorf("pausing nebula scheduled backup [%s/%s] successful", scheduledBackup.Namespace, scheduledBackup.Name)
}

func (bm *scheduledBackupManager) Resume(scheduledBackup *v1alpha1.NebulaScheduledBackup) error {
	var nextBackupTime *metav1.Time
	var err error
	currTime := metav1.Now()
	if scheduledBackup.Status.NextBackupTime == nil || scheduledBackup.Status.NextBackupTime.Before(&currTime) {
		nextBackupTime, err = calculateNextRunTime(scheduledBackup)
		if err != nil {
			return fmt.Errorf("calculating next runtime for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}
	}

	if err := bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
		NextBackupTime: nextBackupTime,
		Phase:          v1alpha1.ScheduledBackupScheduled,
	}); err != nil {
		return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}
	return utilerrors.ReconcileErrorf("resuming nebula scheduled backup [%s/%s] successful", scheduledBackup.Namespace, scheduledBackup.Name)
}

func (bm *scheduledBackupManager) Sync(scheduledBackup *v1alpha1.NebulaScheduledBackup) error {

	// Detect if schedule for nebula scheduled backup is changed
	if scheduledBackup.Status.CurrSchedule != scheduledBackup.Spec.Schedule {
		klog.Infof("Schedule change detected for scheduled backup %v/%v", scheduledBackup.Namespace, scheduledBackup.Name)
		nextBackupTime, err := calculateNextRunTime(scheduledBackup)
		if err != nil {
			return fmt.Errorf("calculating next runtime for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}

		if err := bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
			CurrSchedule:   scheduledBackup.Spec.Schedule,
			NextBackupTime: nextBackupTime,
		}); err != nil {
			return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}
		return utilerrors.ReconcileErrorf("Schedule for scheduled backup [%s/%s] updated successfully to %s.", scheduledBackup.Namespace, scheduledBackup.Name, scheduledBackup.Spec.Schedule)
	} else if scheduledBackup.Status.Phase == v1alpha1.ScheduledBackupRunning {
		klog.Infof("Running nebula backup detected for scheduled backup %v/%v", scheduledBackup.Namespace, scheduledBackup.Name)
		// Detect if a running backup has completed or has failed
		nebulaBackup, err := bm.clientSet.NebulaBackup().GetNebulaBackup(scheduledBackup.Namespace, fmt.Sprintf("%s-%s-%v", BackupPrefix, scheduledBackup.Name, scheduledBackup.Status.NextBackupTime.Unix()))
		if err != nil {
			return fmt.Errorf("getting running nebula backup for Nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}

		// Detect failed nebula backup
		for _, condition := range nebulaBackup.Status.Conditions {
			if (condition.Type == v1alpha1.BackupFailed || condition.Type == v1alpha1.BackupInvalid) && condition.Status == corev1.ConditionTrue {
				if err := bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
					Phase: v1alpha1.ScheduledBackupJobFailed,
				}); err != nil {
					return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
				}
				return utilerrors.ReconcileErrorf("nebula backup for scheduled backup [%s/%s] failed to complete. reason: %s message: %s", scheduledBackup.Namespace, scheduledBackup.Name, condition.Reason, condition.Message)
			}
		}

		// Detect complete nebula backup
		if condutil.IsBackupComplete(nebulaBackup) {
			if err := bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
				Phase: v1alpha1.ScheduledBackupScheduled,
			}); err != nil {
				return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
			return utilerrors.ReconcileErrorf("nebula backup for scheduled backup [%s/%s] complete succesfully.", scheduledBackup.Namespace, scheduledBackup.Name)
		}
		return utilerrors.ReconcileErrorf("waiting for nebula backup [%s/%s] for scheduled backup [%s/%s] to complete.", nebulaBackup.Namespace, nebulaBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
	} else {
		currTime := metav1.Now()
		if currTime.Equal(scheduledBackup.Status.NextBackupTime) || currTime.After(scheduledBackup.Status.NextBackupTime.Time) {
			klog.Infof("next scheduled backup is up for Nebula scheduled backup %v/%v. Triggering Nebula backup job.", scheduledBackup.Namespace, scheduledBackup.Name)
			lastBackupTime := scheduledBackup.Status.NextBackupTime
			nextBackupTime, err := calculateNextRunTime(scheduledBackup)
			if err != nil {
				return fmt.Errorf("calculating next runtime for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
			if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
				LastBackupTime: lastBackupTime,
				NextBackupTime: nextBackupTime,
				Phase:          v1alpha1.ScheduledBackupScheduled,
			}); err != nil {
				return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}

			nebulaBackup := v1alpha1.NebulaBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%v", BackupPrefix, scheduledBackup.Name, lastBackupTime.Unix()),
					Namespace: scheduledBackup.Namespace,
				},
				Spec: scheduledBackup.Spec.BackupTemplate,
			}

			nebulaBackupManager := nebulabackup.NewBackupManager(bm.clientSet)
			err = nebulaBackupManager.Create(&nebulaBackup)
			if err != nil {
				return fmt.Errorf("trigger nebula backup for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}

			return utilerrors.ReconcileErrorf("nebula backup %s/%s for nebula scheduled backup [%s/%s] triggered succesfully.", nebulaBackup.Namespace, nebulaBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
		}
		return utilerrors.ReconcileErrorf("waiting for nebula scheduled backup [%s/%s] to run next backup", scheduledBackup.Namespace, scheduledBackup.Name)
	}
}

func calculateNextRunTime(scheduledBackup *v1alpha1.NebulaScheduledBackup) (*metav1.Time, error) {
	schedule, err := cron.ParseStandard(scheduledBackup.Spec.Schedule)
	if err != nil {
		return nil, fmt.Errorf("get schedule for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}

	var timeToUse metav1.Time
	currTime := metav1.Now()
	if scheduledBackup.Status.NextBackupTime == nil || scheduledBackup.Status.NextBackupTime.IsZero() {
		timeToUse = currTime
	} else {
		timeToUse = *scheduledBackup.Status.NextBackupTime
	}

	nextBackupTime := metav1.NewTime(schedule.Next(timeToUse.Local()))
	if nextBackupTime.Before(&currTime) {
		nextBackupTime = metav1.NewTime(schedule.Next(currTime.Local()))
	}

	return &nextBackupTime, nil
}
