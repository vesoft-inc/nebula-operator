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

	"github.com/aws/aws-sdk-go/aws"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/br"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	EnvS3AccessKeyName = "S3_ACCESS_KEY"
	EnvS3SecretKeyName = "S3_SECRET_KEY"
)

type Manager interface {
	// Create creates a backup job.
	Create(backup *v1alpha1.NebulaScheduledBackup) error

	// Sync	implements the logic for syncing NebulaBackup.
	Sync(backup *v1alpha1.NebulaScheduledBackup) error
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

	scheduledBackupJob := generateBackupCronjob(scheduledBackup, nc.GetMetadThriftConnAddress())
	if err = bm.clientSet.CronJob().CreateCronJob(scheduledBackupJob); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create backup cronjob err: %w", err)
	}

	if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
		LastBackup:     "N/A",
		LastBackupTime: &metav1.Time{},
	}); err != nil {
		return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}
	return nil
}

func (bm *scheduledBackupManager) Sync(scheduledBackup *v1alpha1.NebulaScheduledBackup) error {
	backupCronJob, err := bm.clientSet.CronJob().GetCronJob(scheduledBackup.Namespace, scheduledBackup.Name)
	if err != nil {
		return fmt.Errorf("get backup cronjob %s/%s err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}

	if backupCronJob.Spec.Suspend != &scheduledBackup.Spec.Pause {
		backupCronJob.Spec.Suspend = &scheduledBackup.Spec.Pause
		err := bm.clientSet.CronJob().UpdateCronJob(backupCronJob)
		if scheduledBackup.Spec.Pause {
			if err != nil {
				return utilerrors.ReconcileErrorf("pausing backup cronjob [%s/%s] failed: %v", backupCronJob.Namespace, backupCronJob.Name, err)
			}
			if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
				Phase: v1alpha1.ScheduledBackupPaused,
			}); err != nil {
				return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
			return utilerrors.ReconcileErrorf("pausing backup cronjob [%s/%s] successful", backupCronJob.Namespace, backupCronJob.Name)
		} else {
			if err != nil {
				return utilerrors.ReconcileErrorf("resuming backup cronjob [%s/%s] failed: %v", backupCronJob.Namespace, backupCronJob.Name, err)
			}
			if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
				Phase: v1alpha1.ScheduledBackupScheduled,
			}); err != nil {
				return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
			return utilerrors.ReconcileErrorf("resuming backup cronjob [%s/%s] successful", backupCronJob.Namespace, backupCronJob.Name)
		}
	}

	if backupCronJob.Spec.Schedule != scheduledBackup.Spec.Schedule {
		backupCronJob.Spec.Schedule = scheduledBackup.Spec.Schedule
		err := bm.clientSet.CronJob().UpdateCronJob(backupCronJob)
		if err != nil {
			return utilerrors.ReconcileErrorf("updating backup cronjob [%s/%s] failed: %v", backupCronJob.Namespace, backupCronJob.Name, err)
		}
	}

	if backupCronJob.Status.Active != nil {
		if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
			LastBackup:     backupCronJob.Name,
			LastBackupTime: backupCronJob.Status.LastScheduleTime,
			Phase:          v1alpha1.ScheduledBackupRunning,
		}); err != nil {
			return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}

		currentBackupJob, err := bm.clientSet.Job().GetJob(backupCronJob.Status.Active[0].Namespace, backupCronJob.Status.Active[0].Name)
		if err != nil {
			return fmt.Errorf("error getting backup job associated with scheduled backup %s/%s: %v", scheduledBackup.Namespace, scheduledBackup.Name, err)
		}

		for _, condition := range currentBackupJob.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				if err = bm.clientSet.NebulaScheduledBackup().UpdateNebulaScheduledBackupStatus(scheduledBackup, &kube.ScheduledBackupUpdateStatus{
					Phase: v1alpha1.ScheduledBackupJobFailed,
				}); err != nil {
					return fmt.Errorf("update nebula scheduled backup %s/%s status err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
				}
				return fmt.Errorf("current running backup failed, reason: %s, message: %s", condition.Reason, condition.Message)
			}
		}

		return utilerrors.ReconcileErrorf("waiting for current running backup [%s/%s] to run", currentBackupJob.Namespace, currentBackupJob.Name)
	}

	return nil
}

func generateBackupCronjob(scheduledBackup *v1alpha1.NebulaScheduledBackup, metaAddr string) *batchv1.CronJob {
	bpCmdHead := "exec /usr/local/bin/br-ent backup full"
	if scheduledBackup.Spec.BackupTemplate.BR.BackupName != "" {
		bpCmdHead = fmt.Sprintf("exec /usr/local/bin/br-ent backup incr --base %s", scheduledBackup.Spec.BackupTemplate.BR.BackupName)
	}

	backupCmd := fmt.Sprintf("%s --meta %s --storage s3://%s --s3.access_key $%s --s3.secret_key $%s --s3.region %s --s3.endpoint %s",
		bpCmdHead, metaAddr, scheduledBackup.Spec.BackupTemplate.BR.S3.Bucket, EnvS3AccessKeyName, EnvS3SecretKeyName, scheduledBackup.Spec.BackupTemplate.BR.S3.Region, scheduledBackup.Spec.BackupTemplate.BR.S3.Endpoint)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scheduledBackup.Name,
			Namespace: scheduledBackup.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          scheduledBackup.Spec.Schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			Suspend:           &scheduledBackup.Spec.Pause,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      scheduledBackup.Name,
					Namespace: scheduledBackup.Namespace,
				},
				Spec: batchv1.JobSpec{
					Parallelism:  pointer.Int32(1),
					Completions:  pointer.Int32(1),
					BackoffLimit: pointer.Int32(0),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ImagePullSecrets: scheduledBackup.Spec.BackupTemplate.ImagePullSecrets,
							Containers: []corev1.Container{
								{
									Name:            "scheduledBackup",
									Image:           scheduledBackup.Spec.BackupTemplate.ToolImage,
									ImagePullPolicy: scheduledBackup.Spec.BackupTemplate.ImagePullPolicy,
									Env: []corev1.EnvVar{
										{
											Name: EnvS3AccessKeyName,
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: scheduledBackup.Spec.BackupTemplate.BR.S3.SecretName,
													},
													Key: br.S3AccessKey,
												},
											},
										},
										{
											Name: EnvS3SecretKeyName,
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: scheduledBackup.Spec.BackupTemplate.BR.S3.SecretName,
													},
													Key: br.S3SecretKey,
												},
											},
										},
									},
									Command: []string{"/bin/sh", "-ecx"},
									Args:    []string{backupCmd},
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
							NodeSelector:  scheduledBackup.Spec.BackupTemplate.NodeSelector,
						},
					},
					TTLSecondsAfterFinished: aws.Int32(600),
				},
			},
		},
	}
}
