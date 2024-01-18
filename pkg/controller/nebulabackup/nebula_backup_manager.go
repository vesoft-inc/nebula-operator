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
	EnvS3AccessKeyName = "AWS_ACCESS_KEY_ID"
	EnvS3SecretKeyName = "AWS_SECRET_ACCESS_KEY"
)

type Manager interface {
	// Create creates a backup job.
	Create(backup *v1alpha1.NebulaBackup) error

	// Sync	implements the logic for syncing NebulaBackup.
	Sync(backup *v1alpha1.NebulaBackup) error
}

var _ Manager = (*backupManager)(nil)

type backupManager struct {
	clientSet kube.ClientSet
}

func NewBackupManager(clientSet kube.ClientSet) Manager {
	return &backupManager{clientSet: clientSet}
}

func (bm *backupManager) Create(backup *v1alpha1.NebulaBackup) error {
	ns := backup.GetNamespace()
	if backup.Spec.BR.ClusterNamespace != nil {
		ns = *backup.Spec.BR.ClusterNamespace
	}

	nc, err := bm.clientSet.NebulaCluster().GetNebulaCluster(ns, backup.Spec.BR.ClusterName)
	if err != nil {
		return fmt.Errorf("get nebula cluster %s/%s err: %w", ns, backup.Spec.BR.ClusterName, err)
	}

	if !nc.IsReady() {
		return fmt.Errorf("nebula cluster %s/%s is not ready", ns, backup.Spec.BR.ClusterName)
	}

	backupJob := bm.generateBackupJob(backup, fmt.Sprintf("%v:%v", nc.MetadComponent().GetPodFQDN(0), nc.MetadComponent().GetPort(v1alpha1.MetadPortNameThrift)))
	if err = bm.clientSet.Job().CreateJob(backupJob); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create backup job err: %w", err)
	}

	if err = bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
		Type:   v1alpha1.BackupRunning,
		Status: corev1.ConditionTrue,
		Reason: "CreateBackupJobSuccess",
	}, &kube.BackupUpdateStatus{
		TimeStarted:   &metav1.Time{Time: time.Now()},
		ConditionType: v1alpha1.BackupRunning,
	}); err != nil {
		return fmt.Errorf("update nebula backup %s/%s status err: %w", backup.Namespace, backup.Name, err)
	}
	return nil
}

func (bm *backupManager) Sync(backup *v1alpha1.NebulaBackup) error {
	backupJob, err := bm.clientSet.Job().GetJob(backup.Namespace, backup.Name)
	if err != nil {
		return fmt.Errorf("get backup job %s/%s err: %w", backup.Namespace, backup.Name, err)
	}

	if backupJob.Status.CompletionTime != nil {
		if err = bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupComplete,
			Status: corev1.ConditionTrue,
			Reason: "BackupComplete",
		}, &kube.BackupUpdateStatus{
			TimeCompleted: &metav1.Time{Time: backupJob.Status.CompletionTime.Time},
			ConditionType: v1alpha1.BackupComplete,
		}); err != nil {
			return fmt.Errorf("update nebula backup %s/%s status err: %w", backup.Namespace, backup.Name, err)
		}
		return nil
	}

	for _, condition := range backupJob.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return fmt.Errorf("backup failed, reason: %s, message: %s", condition.Reason, condition.Message)
		}
	}

	return utilerrors.ReconcileErrorf("waiting for backup job [%s/%s] done", backup.Namespace, backup.Name)
}

func (bm *backupManager) generateBackupJob(backup *v1alpha1.NebulaBackup, metaAddr string) *batchv1.Job {
	var podSpec corev1.PodSpec
	if len(backup.OwnerReferences) != 0 && backup.OwnerReferences[0].Kind == "NebulaScheduledBackup" && backup.Spec.ReservedTimeEpoch != nil || backup.Spec.NumBackupsKeep != nil {
		var maxReservedTimeToUse time.Duration = 0
		if backup.Spec.ReservedTimeEpoch != nil {
			maxReservedTimeToUse = *backup.Spec.ReservedTimeEpoch
		}

		maxBackupToUse := int32(0)
		if backup.Spec.NumBackupsKeep != nil {
			maxBackupToUse = *backup.Spec.NumBackupsKeep
		}

		podSpec = corev1.PodSpec{
			ImagePullSecrets: backup.Spec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Name:            "backup",
					Image:           backup.Spec.ToolImage,
					ImagePullPolicy: backup.Spec.ImagePullPolicy,
					Env: []corev1.EnvVar{
						{
							Name:  "META_ADDRESS",
							Value: metaAddr,
						},
						{
							Name:  "STORAGE",
							Value: fmt.Sprintf("s3://%v", backup.Spec.BR.S3.Bucket),
						},
						{
							Name: EnvS3AccessKeyName,
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: backup.Spec.BR.S3.SecretName,
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
										Name: backup.Spec.BR.S3.SecretName,
									},
									Key: br.S3SecretKey,
								},
							},
						},
						{
							Name:  "S3_REGION",
							Value: backup.Spec.BR.S3.Region,
						},
						{
							Name:  "S3_ENDPOINT",
							Value: backup.Spec.BR.S3.Endpoint,
						},
						{
							Name:  "NUM_BACKUPS_KEEP",
							Value: fmt.Sprintf("%v", maxBackupToUse),
						},
						{
							Name:  "RESERVED_TIME_EPOCH",
							Value: fmt.Sprintf("%v", maxReservedTimeToUse.Seconds()),
						},
					},
					Command: []string{"/bin/bash", "-c"},
					Args:    []string{"/usr/local/bin/runnable/backup-cleanup-run.sh"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "backup-cleanup-volume-runnable",
							MountPath: "/usr/local/bin/runnable",
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "backup-init",
					Image:           backup.Spec.ToolImage,
					ImagePullPolicy: backup.Spec.ImagePullPolicy,
					Command:         []string{"/bin/bash", "-c"},
					Args:            []string{"cp /usr/local/bin/backup-cleanup.sh /usr/local/bin/runnable/backup-cleanup-run.sh; chmod +x /usr/local/bin/runnable/backup-cleanup-run.sh"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "backup-cleanup-volume",
							MountPath: "/usr/local/bin/backup-cleanup.sh",
							SubPath:   "backup-cleanup.sh",
						},
						{
							Name:      "backup-cleanup-volume-runnable",
							MountPath: "/usr/local/bin/runnable",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "backup-cleanup-volume",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("backup-scripts-cm-%v", backup.OwnerReferences[0].Name)},
						},
					},
				},
				{
					Name: "backup-cleanup-volume-runnable",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  backup.Spec.NodeSelector,
		}
	} else {
		bpCmdHead := "exec /usr/local/bin/br-ent backup full"
		if backup.Spec.BR.BackupName != "" {
			bpCmdHead = fmt.Sprintf("exec /usr/local/bin/br-ent backup incr --base %s", backup.Spec.BR.BackupName)
		}

		backupCmd := fmt.Sprintf("%s --meta %s --storage s3://%s --s3.access_key $%s --s3.secret_key $%s --s3.region %s --s3.endpoint %s",
			bpCmdHead, metaAddr, backup.Spec.BR.S3.Bucket, EnvS3AccessKeyName, EnvS3SecretKeyName, backup.Spec.BR.S3.Region, backup.Spec.BR.S3.Endpoint)

		podSpec = corev1.PodSpec{
			ImagePullSecrets: backup.Spec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Name:            "backup",
					Image:           backup.Spec.ToolImage,
					ImagePullPolicy: backup.Spec.ImagePullPolicy,
					Env: []corev1.EnvVar{
						{
							Name: EnvS3AccessKeyName,
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: backup.Spec.BR.S3.SecretName,
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
										Name: backup.Spec.BR.S3.SecretName,
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
			NodeSelector:  backup.Spec.NodeSelector,
		}
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:  pointer.Int32(1),
			Completions:  pointer.Int32(1),
			BackoffLimit: pointer.Int32(0),
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
			TTLSecondsAfterFinished: pointer.Int32(600),
		},
	}
}
