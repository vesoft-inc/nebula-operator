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
	"sort"
	"time"

	"github.com/robfig/cron"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

const (
	BackupPrefix = "nsb"
)

type Manager interface {
	// CleanupFinishedNBJobs implements the logic for deleting all finished Nebula Backup jobs associated with the NebulaScheduledBackup
	CleanupFinishedNBs(backup *v1alpha1.NebulaScheduledBackup, successfulJobs, FailedJobs []v1alpha1.NebulaBackup) error

	// SyncNebulaScheduledBackup implements the logic for syncing a NebulaScheduledBackup.
	SyncNebulaScheduledBackup(backup *v1alpha1.NebulaScheduledBackup, successfulBackups, failedBackups, runningBackups []v1alpha1.NebulaBackup, now *metav1.Time) (*kube.ScheduledBackupUpdateStatus, *metav1.Time, error)
}

var _ Manager = (*scheduledBackupManager)(nil)

type scheduledBackupManager struct {
	clientSet kube.ClientSet
}

func NewBackupManager(clientSet kube.ClientSet) Manager {
	return &scheduledBackupManager{clientSet: clientSet}
}

func (bm *scheduledBackupManager) CleanupFinishedNBs(scheduledBackup *v1alpha1.NebulaScheduledBackup, successfulJobs, failedJobs []v1alpha1.NebulaBackup) error {
	klog.Info("Cleaning up previous successful and failed backup jobs")
	if scheduledBackup.Spec.MaxSuccessfulNebulaBackupJobs == nil && scheduledBackup.Spec.MaxFailedNebulaBackupJobs == nil {
		klog.Info("No limit set for either successful backup jobs or failed backup jobs. Nothing to cleanup.")
		return nil
	}

	var jobsToDelete int32
	if scheduledBackup.Spec.MaxSuccessfulNebulaBackupJobs != nil {
		jobsToDelete = int32(len(successfulJobs)) - *scheduledBackup.Spec.MaxSuccessfulNebulaBackupJobs
		if jobsToDelete > 0 {
			err := bm.cleanupNBs(scheduledBackup, successfulJobs, jobsToDelete)
			if err != nil {
				return fmt.Errorf("cleanup successful backup jobs failed for nebula scheduled backup %s/%s. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
		} else {
			klog.Infof("Number of successful backup jobs is less then %v. Nothing to cleanup", scheduledBackup.Spec.MaxSuccessfulNebulaBackupJobs)
		}
	}

	if scheduledBackup.Spec.MaxFailedNebulaBackupJobs != nil {
		jobsToDelete = int32(len(failedJobs)) - *scheduledBackup.Spec.MaxFailedNebulaBackupJobs
		if jobsToDelete > 0 {
			err := bm.cleanupNBs(scheduledBackup, failedJobs, jobsToDelete)
			if err != nil {
				return fmt.Errorf("cleanup failed backup jobs failed for nebula scheduled backup %s/%s. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}
		} else {
			klog.Infof("Number of failed backup jobs is less then %v. Nothing to cleanup", scheduledBackup.Spec.MaxFailedNebulaBackupJobs)
		}
	}

	return nil
}

func (bm *scheduledBackupManager) SyncNebulaScheduledBackup(scheduledBackup *v1alpha1.NebulaScheduledBackup, successfulBackups, failedBackups, runningBackups []v1alpha1.NebulaBackup, now *metav1.Time) (*kube.ScheduledBackupUpdateStatus, *metav1.Time, error) {
	// Create or undate backup cleanup scripts config map. Needed to clean up previous backups
	nbConfigMap := generateBackupCleanupScriptsConfigMap(scheduledBackup)
	err := bm.clientSet.ConfigMap().CreateOrUpdateConfigMap(nbConfigMap)
	if err != nil {
		return nil, nil, fmt.Errorf("create or update config map for backup cleanup scripts err: %w", err)
	}

	// Don't do anything except update backup cleanup scripts config map if cronjob is paused
	if *scheduledBackup.Spec.Pause {
		klog.Infof("Nebula scheduled backup %s/%s is paused.", scheduledBackup.Namespace, scheduledBackup.Name)
		return nil, nil, nil
	}

	// Check status of most recent job
	klog.Info("Checking status of most recent job")
	var lastSuccessfulTime *metav1.Time
	var lastBackupFailed bool

	if len(successfulBackups) != 0 && len(failedBackups) != 0 {
		sort.Sort(byBackupStartTime(successfulBackups))
		sort.Sort(byBackupStartTime(failedBackups))

		lastSuccessfulBackup := successfulBackups[len(successfulBackups)-1]
		lastFailedBackup := failedBackups[len(failedBackups)-1]
		if lastSuccessfulBackup.Status.TimeStarted.After(lastFailedBackup.Status.TimeStarted.Local()) {
			// Most recent backup job succeeded
			lastSuccessfulTime = lastSuccessfulBackup.Status.TimeCompleted
			klog.Infof("Most recent backup job %s/%s for Nebula scheduled backup %s/%s succeeded", lastSuccessfulBackup.Namespace, lastSuccessfulBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
		} else {
			// Most recent backup job failed
			lastBackupFailed = true
			klog.Errorf("Most recent backup job %s/%s for Nebula scheduled backup %s/%s failed", lastFailedBackup.Namespace, lastFailedBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
		}
	} else if len(successfulBackups) != 0 {
		lastSuccessfulBackup := successfulBackups[len(successfulBackups)-1]
		lastSuccessfulTime = lastSuccessfulBackup.Status.TimeCompleted
		klog.Infof("Most recent backup job %s/%s for Nebula scheduled backup %s/%s succeeded", lastSuccessfulBackup.Namespace, lastSuccessfulBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
	} else if len(failedBackups) != 0 {
		lastFailedBackup := failedBackups[len(failedBackups)-1]
		lastBackupFailed = true
		klog.Errorf("Most recent backup job %s/%s for Nebula scheduled backup %s/%s failed", lastFailedBackup.Namespace, lastFailedBackup.Name, scheduledBackup.Namespace, scheduledBackup.Name)
	} else {
		klog.Info("No previous successful or failed backup jobs detected.")
	}

	// Check Nebula Graph cluster status and return error if it's down.
	ns := scheduledBackup.GetNamespace()
	if scheduledBackup.Spec.BackupTemplate.BR.ClusterNamespace != nil {
		ns = *scheduledBackup.Spec.BackupTemplate.BR.ClusterNamespace
	}

	nc, err := bm.clientSet.NebulaCluster().GetNebulaCluster(ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("get nebula cluster %s/%s err: %w", ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName, err)
	}

	if !nc.IsReady() {
		return nil, nil, fmt.Errorf("nebula cluster %s/%s is not ready", ns, scheduledBackup.Spec.BackupTemplate.BR.ClusterName)
	}

	// Calculate previous and next backup time
	previousBackupTime, nextBackupTime, err := calculateRunTimes(scheduledBackup, now)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate next backup time for Nebula scheduled backup %s/%s. Err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}

	if now.After(previousBackupTime.Local()) {
		if len(runningBackups) > 0 {
			// active running backup from last iteration has not completed. Issue warning and do not start new backup
			klog.Warning("Active backup job from last scheduled run detected for Nebula scheduled backup %s/%s. Will not kick off new backup job. Please consider decreasing the backup frequency by adjusting the schedule.", scheduledBackup.Namespace, scheduledBackup.Name)
			if len(runningBackups) != 1 {
				// This could be because someone manually started a nebula backup and associated it to this scheduled backup. Need to issue another warning.
				klog.Warning("More than 1 active backup detected for Nebula scheduled backup %s/%s. Please check if the second job was manually started and associated to this crontab", scheduledBackup.Namespace, scheduledBackup.Name)
			}

			for _, backup := range runningBackups {
				klog.Warningf("Active backup job %v detected", backup.Name)
			}
			return &kube.ScheduledBackupUpdateStatus{
				LastSuccessfulBackupTime: lastSuccessfulTime,
				MostRecentJobFailed:      &lastBackupFailed,
			}, nextBackupTime, nil
		} else {
			klog.Infof("Triggering new Nebula backup job for Nebula Scheduled Backup %s/%s.", scheduledBackup.Namespace, scheduledBackup.Name)

			if scheduledBackup.Spec.MaxRetentionTime != nil {
				maxBackupDuration, err := time.ParseDuration(*scheduledBackup.Spec.MaxRetentionTime)
				if err != nil {
					return nil, nil, fmt.Errorf("parsing maxReservedTime for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
				}
				scheduledBackup.Spec.BackupTemplate.ReservedTimeEpoch = &maxBackupDuration
			}
			scheduledBackup.Spec.BackupTemplate.NumBackupsKeep = scheduledBackup.Spec.MaxBackups

			nebulaBackup := v1alpha1.NebulaBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%v", BackupPrefix, scheduledBackup.Name, previousBackupTime.Unix()),
					Namespace: scheduledBackup.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*getOwnerReferenceForSubresources(scheduledBackup),
					},
				},
				Spec: scheduledBackup.Spec.BackupTemplate,
			}

			err = bm.clientSet.NebulaBackup().CreateNebulaBackup(&nebulaBackup)
			if err != nil {
				return nil, nil, fmt.Errorf("trigger nebula backup for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
			}

			return &kube.ScheduledBackupUpdateStatus{
				LastScheduledBackupTime:  previousBackupTime,
				LastSuccessfulBackupTime: lastSuccessfulTime,
				MostRecentJobFailed:      &lastBackupFailed,
			}, nextBackupTime, nil
		}
	}

	return &kube.ScheduledBackupUpdateStatus{
		LastSuccessfulBackupTime: lastSuccessfulTime,
		MostRecentJobFailed:      &lastBackupFailed,
	}, nextBackupTime, nil
}

func (bm *scheduledBackupManager) cleanupNBs(scheduledBackup *v1alpha1.NebulaScheduledBackup, nebulaBackupJobs []v1alpha1.NebulaBackup, numToDelete int32) error {
	sort.Sort(byBackupStartTime(nebulaBackupJobs))

	for i := 0; i < int(numToDelete); i++ {
		klog.Info("Removing Nebula Backup %v triggered by Nebula Scheduled Backup %v/%v", nebulaBackupJobs[i])
		err := bm.clientSet.NebulaBackup().DeleteNebulaBackup(nebulaBackupJobs[i].Namespace, nebulaBackupJobs[i].Name)
		if err != nil {
			return fmt.Errorf("error deleteing backup job %v/%v for nebula scheduled backup %s/%s. err: %w", nebulaBackupJobs[i].Namespace, nebulaBackupJobs[i].Name, scheduledBackup.Namespace, scheduledBackup.Name, err)
		}
	}

	return nil
}

func getBackupCleanupScriptsObjectKey(scheduledBackup *v1alpha1.NebulaScheduledBackup) client.ObjectKey {
	return client.ObjectKey{
		Namespace: scheduledBackup.Namespace,
		Name:      fmt.Sprintf("backup-scripts-cm-%v", scheduledBackup.Name),
	}
}

func getOwnerReferenceForSubresources(scheduledBackup *v1alpha1.NebulaScheduledBackup) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: scheduledBackup.APIVersion,
		Kind:       scheduledBackup.Kind,
		Name:       scheduledBackup.Name,
		UID:        scheduledBackup.UID,
	}
}

func generateBackupCleanupScriptsConfigMap(scheduledBackup *v1alpha1.NebulaScheduledBackup) *corev1.ConfigMap {
	nbObjectKey := getBackupCleanupScriptsObjectKey(scheduledBackup)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nbObjectKey.Name,
			Namespace: nbObjectKey.Namespace,
			Labels: map[string]string{
				"backup-name":      scheduledBackup.Name,
				"backup-namespace": scheduledBackup.Namespace,
			},
			OwnerReferences: []metav1.OwnerReference{
				*getOwnerReferenceForSubresources(scheduledBackup),
			},
		},

		Data: map[string]string{
			"backup-cleanup.sh": `#!/bin/bash

# Step 1: run backup
echo "Running Nebula backup..."
/usr/local/bin/br-ent backup full --meta "$META_ADDRESS" --storage "$STORAGE" --s3.access_key "$AWS_ACCESS_KEY_ID" --s3.secret_key "$AWS_SECRET_ACCESS_KEY" --s3.region "$S3_REGION" --s3.endpoint "$S3_ENDPOINT"
echo "Nebula backup complete.\n"

# Step 2: get current backups
echo "Getting current backup info..."
backup_names=($(br-ent show --s3.endpoint "$S3_ENDPOINT" --storage "$STORAGE" --s3.access_key "$AWS_ACCESS_KEY_ID" --s3.secret_key "$AWS_SECRET_ACCESS_KEY" --s3.region "$S3_REGION" | grep -e "._[0-9]" | awk -F '|' '{print $2}'))
backup_dates=($(br-ent show --s3.endpoint "$S3_ENDPOINT" --storage "$STORAGE" --s3.access_key "$AWS_ACCESS_KEY_ID" --s3.secret_key "$AWS_SECRET_ACCESS_KEY" --s3.region "$S3_REGION" | grep -e "._[0-9]" | awk -F '|' '{print $3}' | tr " " "T"))
total_backups=$(br-ent show --s3.endpoint "$S3_ENDPOINT" --storage "$STORAGE" --s3.access_key "$AWS_ACCESS_KEY_ID" --s3.secret_key "$AWS_SECRET_ACCESS_KEY" --s3.region "$S3_REGION" | grep -e "._[0-9]" | wc -l)
echo "Current backup info retrieved.\n"


# Step 3: remove backups that need to be removed
echo "Removing previous expired backups..."
if [[ $RESERVED_TIME_EPOCH -gt 0 ]]; then
	echo "Maximum retention time of ${RESERVED_TIME_EPOCH}s set. Removing all previous backups exceeding this retention time...."
	now=$(date +"%s")
	echo "Current time epoach: $now"
	for ind in ${!backup_names[@]}
	do
		nb_date=$(echo "${backup_dates[ind]}" | tr "T" " " | xargs)
		nb_date_epoach=$(date -d "$nb_date" +"%s")
		diff=$((now - nb_date_epoach))
		echo "Backing up file ${backup_names[ind]}. Backup date: \"$nb_date ($nb_date_epoach)\". Diff: \"$diff\""

		if [[ $diff -gt $RESERVED_TIME_EPOCH ]]; then
			echo "File ${backup_names[ind]} is older than the maximum reserved time. Deleting..."
			br-ent cleanup --meta "$META_ADDRESS" --storage "$STORAGE" --s3.access_key $AWS_ACCESS_KEY_ID --s3.secret_key $AWS_SECRET_ACCESS_KEY --s3.region "$S3_REGION" --s3.endpoint "$S3_ENDPOINT" --name "${backup_names[ind]}"
		fi
	done
	echo "All necessary previous backups cleaned up."
elif [[ $NUM_BACKUPS_KEEP -gt 0 ]]; then
	if [[ $total_backups -gt $NUM_BACKUPS_KEEP ]]; then
		echo "Maximum number of backups $NUM_BACKUPS_KEEP set. Removing all backups exceeding this number starting with the oldest backup..."
		num_to_del=$((total_backups - NUM_BACKUPS_KEEP))
		echo "Number of previous backups to delete: $num_to_del"
		for ind in $(seq 0 $((num_to_del - 1)))
		do
			br-ent cleanup --meta "$META_ADDRESS" --storage "$STORAGE" --s3.access_key $AWS_ACCESS_KEY_ID --s3.secret_key $AWS_SECRET_ACCESS_KEY --s3.region "$S3_REGION" --s3.endpoint "$S3_ENDPOINT" --name "${backup_names[ind]}"
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

// calculateRunTimes calculates the next runtime and the most recent run time if the scheduled backup was not paused.
func calculateRunTimes(scheduledBackup *v1alpha1.NebulaScheduledBackup, now *metav1.Time) (*metav1.Time, *metav1.Time, error) {
	schedule, err := cron.ParseStandard(scheduledBackup.Spec.Schedule)
	if err != nil {
		return nil, nil, fmt.Errorf("get schedule for nebula scheduled backup %s/%s failed. err: %w", scheduledBackup.Namespace, scheduledBackup.Name, err)
	}

	earliestTime := scheduledBackup.CreationTimestamp
	if scheduledBackup.Status.LastScheduledBackupTime != nil {
		earliestTime = *scheduledBackup.Status.LastScheduledBackupTime
	}

	nextT1 := metav1.NewTime(schedule.Next(earliestTime.Local()))
	nextT2 := metav1.NewTime(schedule.Next(nextT1.Local()))

	if now.Before(&nextT1) {
		return nil, &nextT1, nil
	}

	if now.Before(&nextT2) {
		return &nextT1, &nextT2, nil
	}

	// Check for invalid cron schedule that'll slide past ParseStandard (i.e "0 0 31 2 *")
	timeBetweenSchedules := nextT2.Sub(nextT1.Local()).Round(time.Second).Seconds()
	if timeBetweenSchedules < 1 {
		return nil, nil, fmt.Errorf("invalid schedule %v", scheduledBackup.Spec.Schedule)
	}

	elapsedTime := now.Sub(nextT1.Local()).Seconds()
	missedRuns := (elapsedTime / timeBetweenSchedules) + 1

	if missedRuns > 80 {
		klog.Warning("too many missed runs (>80). Please check for possible clock skew")
	}

	mostRecentBackupTime := metav1.NewTime(nextT1.Add(time.Duration((missedRuns-1-1)*timeBetweenSchedules) * time.Second))
	for t := schedule.Next(mostRecentBackupTime.Local()); !t.After(now.Local()); t = schedule.Next(t) {
		mostRecentBackupTime = metav1.NewTime(t)
	}

	nextBackupTime := metav1.NewTime(schedule.Next(mostRecentBackupTime.Local()))

	return &mostRecentBackupTime, &nextBackupTime, nil
}

// byBackupStartTime sorts a list of nebula backups by start timestamp, using their names as a tie breaker.
type byBackupStartTime []v1alpha1.NebulaBackup

func (bt byBackupStartTime) Len() int {
	return len(bt)
}

func (bt byBackupStartTime) Swap(i, j int) {
	bt[i], bt[j] = bt[j], bt[i]
}

func (bt byBackupStartTime) Less(i, j int) bool {
	if bt[i].Status.TimeStarted == nil && bt[j].Status.TimeStarted != nil {
		return false
	}
	if bt[i].Status.TimeStarted != nil && bt[j].Status.TimeStarted == nil {
		return true
	}
	if bt[i].Status.TimeStarted.Equal(bt[j].Status.TimeStarted) {
		return bt[i].Name < bt[j].Name
	}
	return bt[i].Status.TimeStarted.Before(bt[j].Status.TimeStarted)
}
