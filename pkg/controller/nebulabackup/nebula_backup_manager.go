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
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/br"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	S3AccessKey = "AWS_ACCESS_KEY_ID"
	S3SecretKey = "AWS_SECRET_ACCESS_KEY"
)

type Manager interface {
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

func (bm *backupManager) Sync(backup *v1alpha1.NebulaBackup) error {

	var nbCondition *v1alpha1.BackupCondition
	var nbStatus *kube.BackupUpdateStatus
	var backupJob *batchv1.Job
	if backup.Status.TimeStarted == nil {
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

		backupJob = bm.generateBackupJob(backup, fmt.Sprintf("%v:%v", nc.MetadComponent().GetPodFQDN(0), nc.MetadComponent().GetPort(v1alpha1.MetadPortNameThrift)))
		if err = bm.clientSet.Job().CreateJob(backupJob); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create backup job err: %w", err)
		}

		nbCondition = &v1alpha1.BackupCondition{
			Type:   v1alpha1.BackupRunning,
			Status: corev1.ConditionTrue,
			Reason: "CreateBackupJobSuccess",
		}

		nbStatus = &kube.BackupUpdateStatus{
			TimeStarted:   &metav1.Time{Time: time.Now()},
			ConditionType: v1alpha1.BackupRunning,
		}

		klog.Infof("Backup job [%s/%s] for NebulaBackup [%s/%s] created successfully", backupJob.Namespace, backupJob.Name, backup.Namespace, backup.Name)

	} else {
		var err error
		backupJob, err = bm.clientSet.Job().GetJob(backup.Namespace, backup.Name)
		if err != nil {
			return fmt.Errorf("get backup job %s/%s err: %w", backup.Namespace, backup.Name, err)
		}

		for _, condition := range backupJob.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				nbCondition = &v1alpha1.BackupCondition{
					Type:   v1alpha1.BackupComplete,
					Status: corev1.ConditionTrue,
					Reason: "BackupComplete",
				}

				nbStatus = &kube.BackupUpdateStatus{
					TimeCompleted: &metav1.Time{Time: backupJob.Status.CompletionTime.Time},
					ConditionType: v1alpha1.BackupComplete,
				}

				if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, nbCondition, nbStatus); err != nil {
					return fmt.Errorf("update nebula backup %s/%s status err: %w", backup.Namespace, backup.Name, err)
				}

				return nil
			} else if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return fmt.Errorf("backup failed, reason: %s, message: %s", condition.Reason, condition.Message)
			}
		}
	}

	if err := bm.clientSet.NebulaBackup().UpdateNebulaBackupStatus(backup, nbCondition, nbStatus); err != nil {
		return fmt.Errorf("update nebula backup %s/%s status err: %w", backup.Namespace, backup.Name, err)
	}
	return utilerrors.ReconcileErrorf("Waiting for backup job [%s/%s] of NebulaBackup [%s/%s] to complete", backupJob.Namespace, backupJob.Name, backup.Namespace, backup.Name)
}

func createStorageString(backup *v1alpha1.NebulaBackup) string {
	var storageString string
	switch backup.Spec.BR.StorageProviderType {
	case "s3":
		storageString = fmt.Sprintf("--storage s3://%s --s3.access_key $(%s) --s3.secret_key $(%s) --s3.region %s --s3.endpoint %s", backup.Spec.BR.S3.Bucket, S3AccessKey, S3SecretKey, backup.Spec.BR.S3.Region, backup.Spec.BR.S3.Endpoint)
	}
	return storageString
}

func getEnvForCredentials(backup *v1alpha1.NebulaBackup) []corev1.EnvVar {
	if os.Getenv(S3AccessKey) != "" && os.Getenv(S3SecretKey) != "" {
		return []corev1.EnvVar{
			{
				Name:  S3AccessKey,
				Value: os.Getenv(S3AccessKey),
			},
			{
				Name:  S3SecretKey,
				Value: os.Getenv(S3SecretKey),
			},
		}
	}

	return []corev1.EnvVar{
		{
			Name: S3AccessKey,
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
			Name: S3SecretKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: backup.Spec.BR.S3.SecretName,
					},
					Key: br.S3SecretKey,
				},
			},
		},
	}
}

func (bm *backupManager) generateBackupJob(backup *v1alpha1.NebulaBackup, metaAddr string) *batchv1.Job {
	var podSpec corev1.PodSpec
	var ttlFinished *int32
	storageString := createStorageString(backup)

	credentials := getEnvForCredentials(backup)

	if len(backup.OwnerReferences) != 0 && backup.OwnerReferences[0].Kind == "NebulaScheduledBackup" && len(backup.Env) != 0 {
		envtoPass := append(credentials, backup.Env...)
		podSpec = corev1.PodSpec{
			ImagePullSecrets: backup.Spec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Name:            "backup",
					Image:           backup.Spec.ToolImage,
					ImagePullPolicy: backup.Spec.ImagePullPolicy,
					Env: append(envtoPass,
						corev1.EnvVar{
							Name:  "META_ADDRESS",
							Value: metaAddr,
						},
						corev1.EnvVar{
							Name:  "STORAGE_LINE",
							Value: storageString,
						},
					),
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

		backupCmd := fmt.Sprintf("%s --meta %s %s", bpCmdHead, metaAddr, storageString)

		podSpec = corev1.PodSpec{
			ImagePullSecrets: backup.Spec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Name:            "backup",
					Image:           backup.Spec.ToolImage,
					ImagePullPolicy: backup.Spec.ImagePullPolicy,
					Env:             credentials,
					Command:         []string{"/bin/sh", "-ecx"},
					Args:            []string{backupCmd},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  backup.Spec.NodeSelector,
		}
		ttlFinished = pointer.Int32(600)
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
			TTLSecondsAfterFinished: ttlFinished,
		},
	}
}
