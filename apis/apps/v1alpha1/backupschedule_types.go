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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupConditionType represents a valid condition of a Backup.
type ScheduledBackupConditionType string

const (
	// ScheduledBackupPending means the scheduled backup job is pending, waiting for creation of the backup cronjob
	ScheduledBackupPending ScheduledBackupConditionType = "Pending"
	// ScheduledBackupScheduled means the scheduled backup cronjob was created successfully and no active backup jobs are running
	// if there was an active backup job, the job has executed successfully and the backup data has been loaded into the nebula cluster.
	ScheduledBackupScheduled ScheduledBackupConditionType = "Scheduled"
	// ScheduledBackupRunning means there's an active backup job current running.
	ScheduledBackupRunning ScheduledBackupConditionType = "Running"
	// ScheduledBackupPaused means the schedule backup is currently suspended
	ScheduledBackupPaused ScheduledBackupConditionType = "Paused"
	// ScheduledBackupJobFailed means the active backup job has failed to execute successfully
	ScheduledBackupJobFailed ScheduledBackupConditionType = "Backup job failed"
	// BackupFailed means the backup cron job creation has failed.
	ScheduledBackupFailed ScheduledBackupConditionType = "Cron Creation Failed"
	// BackupInvalid means invalid backup CR.
	ScheduledBackupInvalid ScheduledBackupConditionType = "Invalid"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="bs"

type NebulaScheduledBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduledBackupSpec   `json:"spec,omitempty"`
	Status ScheduledBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// NebulaScheduledBackupList contains a list of NebulaScheduledBackup.
type NebulaScheduledBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NebulaScheduledBackup `json:"items"`
}

// ScheduledBackupSpec contains the specification for a NebulaScheduledBackup of a nebula cluster NebulaScheduledBackup.
type ScheduledBackupSpec struct {
	// Schedule specifies the cron string used for backup scheduling.
	Schedule string `json:"schedule"`
	// Pause means paused backupSchedule
	Pause bool `json:"pause,omitempty"`
	// MaxBackups is to specify how many backups we want to keep
	// 0 is magic number to indicate un-limited backups.
	// if MaxBackups and MaxReservedTime are set at the same time, MaxReservedTime is preferred
	// and MaxBackups is ignored.
	MaxBackups *int32 `json:"maxBackups,omitempty"`
	// MaxReservedTime is to specify how long backups we want to keep.
	MaxReservedTime *string `json:"maxReservedTime,omitempty"`
	// BackupTemplate is the specification of the backup structure to get scheduled.
	BackupTemplate BackupSpec `json:"backupTemplate"`
	// LogBackupTemplate is the specification of the log backup structure to get scheduled.
}

// ScheduledBackupStatus represents the current status of a nebula cluster NebulaScheduledBackup.
type ScheduledBackupStatus struct {
	// LastBackup represents the last backup.
	LastBackup string `json:"lastBackup,omitempty"`
	// LastBackupTime represents the last time the backup was successfully created.
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`
	// Phase represents the status of the scheduled backup
	Phase ScheduledBackupConditionType `json:"phase,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NebulaScheduledBackup{}, &NebulaScheduledBackupList{})
}
