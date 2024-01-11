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

	"github.com/aws/aws-sdk-go/aws"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	if backup.Spec.MaxReservedTime != nil || backup.Spec.MaxBackups != nil && *backup.Spec.MaxBackups != int32(0) {
		nbConfigMap := generateScheduledBackupScriptsConfigMap(backup)
		err = bm.clientSet.ConfigMap().CreateOrUpdateConfigMap(nbConfigMap)
		if err != nil {
			return fmt.Errorf("create config map for scheduled backup scripts err: %w", err)
		}
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
		if backup.Spec.MaxReservedTime != nil || backup.Spec.MaxBackups != nil && *backup.Spec.MaxBackups != int32(0) {
			err := bm.clientSet.Job().DeleteJob(backupJob.Namespace, backupJob.Name)
			if err != nil {
				return fmt.Errorf("deleting backup job %s/%s err: %w", backupJob.Namespace, backupJob.Name, err)
			}

			objectKey := getScheduledBackupScriptsObjectKey(backup)
			err = bm.clientSet.ConfigMap().DeleteConfigMap(objectKey.Namespace, objectKey.Name)
			if err != nil {
				return fmt.Errorf("deleting config map %s/%s err: %w", objectKey.Namespace, objectKey.Name, err)
			}
		}

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

func getScheduledBackupScriptsObjectKey(backup *v1alpha1.NebulaBackup) client.ObjectKey {
	return client.ObjectKey{
		Namespace: backup.Namespace,
		Name:      fmt.Sprintf("backup-scripts-cm-%v", backup.Name),
	}
}

func generateScheduledBackupScriptsConfigMap(backup *v1alpha1.NebulaBackup) *corev1.ConfigMap {
	nbObjectKey := getScheduledBackupScriptsObjectKey(backup)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nbObjectKey.Name,
			Namespace: nbObjectKey.Namespace,
			Labels: map[string]string{
				"backup-name":      backup.Name,
				"backup-namespace": backup.Namespace,
			},
		},
		Data: map[string]string{ // TODO: remove x. x means no need to configure, but it is a required field in the program.
			"backup-cleanup.sh": `#!/bin/bash
ACCESS_KEY=$S3_ACCESS_KEY
SECRET_KEY=$S3_SECRET_KEY

function usage {
    echo "usage $0 [-h] < -m meta_address, -s storage, -r s3.region, -e s3.endpoint, -b max_backups, -t max_reserved_time >"
    echo "-h displays this message"
    echo "-m specifies the meta server address"
    echo "-s specifies the backup target url"
    echo "-r specifies the s3 bucket region"
    echo "-e specifies the s3 endpoint"
    echo "-b specifies the maximum number of backups"
    echo "-t spefifies the maximum reserved time for backups"
}

function convert_to_sec {
    local to_convert="$1"

    unit=${to_convert: -1}
    num=${to_convert::-1}

    case "${unit}" in
        s)
            echo $num
            ;;
        m)
            echo "$((num * 60))"
            ;;
        h)
            echo "$((num * 60 * 60))"  
            ;;
        d)
            echo "$((num * 24 * 60 * 60))"
            ;;
        w)
            echo "$((num * 7 * 24 * 60 * 60))"
            ;;
        M)
            echo "$((num * 30 * 24 * 60 * 60))"
            ;;
        Y)
            echo "$((num * 365 * 24 * 60 * 60))"
            ;;
        "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9")
            echo $to_convert
            ;;
        *)
            echo "Invalid value $to_convert"
            ;;
    esac
}


while getopts ':hm:s:r:e:b:t:' arg; do
  case "${arg}" in
    h) 
        usage
        exit 0
        ;;
    m) 
        meta_address="${OPTARG}"
        ;;
    s) 
        storage="${OPTARG}" 
        ;;
    r) 
        s3_region="${OPTARG}" 
        ;;
    e)
        s3_endpoint="${OPTARG}"
        ;;
    b)
        max_backups="${OPTARG}"
        ;;
    t)
        max_reserved_time="${OPTARG}"
        ;;
    :) 
        echo "Option -${OPTARG} requires an argument."
        usage
        exit 1 
        ;;
    ?)
        echo "Invalid option: -${OPTARG}."
        usage
        exit 1
        ;;
  esac
done

if [[ -z $meta_address || -z $storage || -z $s3_region || -z $s3_endpoint || -z $max_backups || -z $max_reserved_time ]]; then
    echo "Parameters meta_address, storage, s3_region, s3_endpoint, max_backups and max_reserved_time are all required. Please specify them with the -m, -s, -r, -e, -b and -t flags respectively."
    usage
    exit 2
fi

# Step 1: run backup
echo "Running Nebula backup..."
/usr/local/bin/br-ent backup full --meta "$meta_address" --storage "$storage" --s3.access_key "$ACCESS_KEY" --s3.secret_key "$SECRET_KEY" --s3.region "$s3_region" --s3.endpoint "$s3_endpoint"
echo "Nebula backup complete.\n"

# Step 2: get current backups
echo "Getting current backup info..."
backup_names=($(br-ent show --s3.endpoint "$s3_endpoint" --storage "$storage" --s3.access_key "$ACCESS_KEY" --s3.secret_key "$SECRET_KEY" --s3.region "$s3_region" | grep -e "._[0-9]" | awk -F '|' '{print $2}'))
backup_dates=($(br-ent show --s3.endpoint "$s3_endpoint" --storage "$storage" --s3.access_key "$ACCESS_KEY" --s3.secret_key "$SECRET_KEY" --s3.region "$s3_region" | grep -e "._[0-9]" | awk -F '|' '{print $3}' | tr " " "T"))
total_backups=$(br-ent show --s3.endpoint "$s3_endpoint" --storage "$storage" --s3.access_key "$ACCESS_KEY" --s3.secret_key "$SECRET_KEY" --s3.region "$s3_region" | grep -e "._[0-9]" | wc -l)
echo "Current backup info retrieved.\n"


# Step 3: remove backups that need to be removed
echo "Removing previous expired backups..."
max_reserved_time_epoach=$(convert_to_sec $max_reserved_time)
if [[ $max_reserved_time_epoach -gt 0 ]]; then
    echo "Maximum retention time of $max_reserved_time set. Removing all previous backups exceeding this retention time...."
    now=$(date +"%s")
    echo "Maximum retention time epoach: $max_reserved_time_epoach"
    echo "Current time epoach: $now"
    for ind in ${!backup_names[@]}
    do
        nb_date=$(echo "${backup_dates[ind]}" | tr "T" " " | xargs)
        nb_date_epoach=$(date -d "$nb_date" +"%s")
        diff=$((now - nb_date_epoach))
        echo "Backing up file ${backup_names[ind]}. Backup date: \"$nb_date ($nb_date_epoach)\". Diff: \"$diff\""

        if [[ $diff -gt $max_reserved_time_epoach ]]; then
            echo "File ${backup_names[ind]} is older than the maximum reserved time. Deleting..."
            br-ent cleanup --meta "$meta_address" --storage "$storage" --s3.access_key $ACCESS_KEY --s3.secret_key $SECRET_KEY --s3.region "$s3_region" --s3.endpoint "$s3_endpoint" --name "${backup_names[ind]}"
        fi
    done
    echo "All necessary previous backups cleaned up."
elif [[ $max_backups -gt 0 ]]; then
    if [[ $total_backups -gt $max_backups ]]; then
        echo "Maximum number of backups $max_backups set. Removing all backups exceeding this number starting with the oldest backup..."
        num_to_del=$((total_backups - max_backups - 1))
        echo "Number of previous backups to delete: $num_to_del"
        for ind in $(seq 0 $num_to_del)
        do
            br-ent cleanup --meta "$meta_address" --storage "$storage" --s3.access_key $ACCESS_KEY --s3.secret_key $SECRET_KEY --s3.region "$s3_region" --s3.endpoint "$s3_endpoint" --name "${backup_names[ind]}"
        done
        echo "All necessary previous backups cleaned up."
    else
        echo "Current number of backups has not exceeded the maximum number of backups to be kept. Will leave all previous backups alone."
    fi
else
    echo "No max retention time or maximum number of backups set. Will leave all previous backups alone"
fi
`,
		},
	}
}

func (bm *backupManager) generateBackupJob(backup *v1alpha1.NebulaBackup, metaAddr string) *batchv1.Job {
	if backup.Spec.MaxReservedTime != nil || backup.Spec.MaxBackups != nil && *backup.Spec.MaxBackups != int32(0) {
		maxReservedTimeToUse := "0"
		if backup.Spec.MaxReservedTime != nil {
			maxReservedTimeToUse = *backup.Spec.MaxReservedTime
		}

		maxBackupToUse := int32(0)
		if backup.Spec.MaxBackups != nil {
			maxBackupToUse = *backup.Spec.MaxBackups
		}

		backupCmd := fmt.Sprintf("/usr/local/bin/runnable/backup-cleanup-run.sh -m %v -s s3://%v -r %v -e %v -b %v -t %v",
			metaAddr, backup.Spec.BR.S3.Bucket, backup.Spec.BR.S3.Region, backup.Spec.BR.S3.Endpoint, maxBackupToUse, maxReservedTimeToUse,
		)
		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backup.Name,
				Namespace: backup.Namespace,
			},
			Spec: batchv1.JobSpec{
				Parallelism:  pointer.Int32(1),
				Completions:  pointer.Int32(1),
				BackoffLimit: pointer.Int32(0),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
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
								Command: []string{"/bin/bash", "-c"},
								Args:    []string{backupCmd},
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
										LocalObjectReference: corev1.LocalObjectReference{Name: getScheduledBackupScriptsObjectKey(backup).Name},
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
					},
				},
				TTLSecondsAfterFinished: aws.Int32(600),
			},
		}
	} else {
		bpCmdHead := "exec /usr/local/bin/br-ent backup full"
		if backup.Spec.BR.BackupName != "" {
			bpCmdHead = fmt.Sprintf("exec /usr/local/bin/br-ent backup incr --base %s", backup.Spec.BR.BackupName)
		}

		backupCmd := fmt.Sprintf("%s --meta %s --storage s3://%s --s3.access_key $%s --s3.secret_key $%s --s3.region %s --s3.endpoint %s",
			bpCmdHead, metaAddr, backup.Spec.BR.S3.Bucket, EnvS3AccessKeyName, EnvS3SecretKeyName, backup.Spec.BR.S3.Region, backup.Spec.BR.S3.Endpoint)

		return &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backup.Name,
				Namespace: backup.Namespace,
			},
			Spec: batchv1.JobSpec{
				Parallelism:  pointer.Int32(1),
				Completions:  pointer.Int32(1),
				BackoffLimit: pointer.Int32(0),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
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
					},
				},
				TTLSecondsAfterFinished: aws.Int32(600),
			},
		}
	}
}
