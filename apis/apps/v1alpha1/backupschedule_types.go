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

var (
	maxSuccessfulBackupJobsDef int32 = 3
	maxSuccessfulFailedJobsDef int32 = 3
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="nsb"
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`,description="The current schedule set for the scheduled backup"
// +kubebuilder:printcolumn:name="Pause",type=string,JSONPath=`.spec.pause`,description="Whether or not the scheduled backup is paused"
// +kubebuilder:printcolumn:name="Last Triggered Backup",type=string,JSONPath=`.status.lastScheduledBackupTime`,description="The timestamp at which the last backup was triggered"
// +kubebuilder:printcolumn:name="Last Successful Backup",format=date-time,type=string,JSONPath=`.status.lastSuccessfulBackupTime`,description="The timestamp at which the last backup was successful completed"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
	Pause *bool `json:"pause,omitempty"`
	// MaxBackups specifies how many backups we want to keep in the remote storage bucket.
	// 0 is the magic number to indicate unlimited backups.
	// if both MaxBackups and MaxReservedTime are set at the same time, MaxReservedTime will be used
	// and MaxBackups is ignored.
	MaxBackups *int32 `json:"maxBackups,omitempty"`
	// MaxRetentionTime specifies how long we want the backups in the remote storage bucket to be kept for.
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(s|m|h))+$`
	MaxRetentionTime *string `json:"maxRetentionTime,omitempty"`
	// BackupTemplate is the specification of the backup structure to schedule.
	BackupTemplate BackupSpec `json:"backupTemplate"`
	// MaxSuccessfulNebulaBackupJobs specifies the maximum number of successful backup jobs to keep. Default 3.
	MaxSuccessfulNebulaBackupJobs *int32 `json:"maxSuccessfulNebulaBackupJobs,omitempty"`
	// MaxFailedNebulaBackupJobs specifies the maximum number of failed backup jobs to keep. Default 3
	MaxFailedNebulaBackupJobs *int32 `json:"maxFailedNebulaBackupJobs,omitempty"`
}

// ScheduledBackupStatus represents the current status of a nebula cluster NebulaScheduledBackup.
type ScheduledBackupStatus struct {
	// CurrPauseStatus represent the current pause status of the nebula scheduled backup
	CurrPauseStatus *bool `json:"currPauseStatus,omitempty"`
	// LastBackup represents the last backup. Used for scheduled incremental backups. Not supported for now.
	//LastBackup string `json:"lastBackup,omitempty"`
	// LastScheduledBackupTime represents the last time a backup job was successfully scheduled.
	LastScheduledBackupTime *metav1.Time `json:"lastScheduledBackupTime,omitempty"`
	// LastSuccessfulBackupTime represents the last time a backup was successfully created.
	LastSuccessfulBackupTime *metav1.Time `json:"lastSuccessfulBackupTime,omitempty"`
	// NumberOfSuccessfulBackups represents the total number of successful Nebula Backups run by this the Nebula Scheduled Backup
	NumberOfSuccessfulBackups *int32 `json:"numberofSuccessfulBackups,omitempty"`
	// NumberOfFailedBackups represents the total number of failed Nebula Backups run by this the Nebula Scheduled Backup
	NumberOfFailedBackups *int32 `json:"numberofFailedBackups,omitempty"`
	// MostRecentJobFailed represents if the most recent backup job failed.
	MostRecentJobFailed *bool `json:"mostRecentJobFailed,omitempty"`
}

// Defaulting implementation for ScheduledBackupStatus
func (nsb *NebulaScheduledBackup) Default() {
	if nsb.Spec.MaxSuccessfulNebulaBackupJobs == nil {
		nsb.Spec.MaxSuccessfulNebulaBackupJobs = &maxSuccessfulBackupJobsDef
	}
	if nsb.Spec.MaxFailedNebulaBackupJobs == nil {
		nsb.Spec.MaxFailedNebulaBackupJobs = &maxSuccessfulFailedJobsDef
	}
}

func init() {
	SchemeBuilder.Register(&NebulaScheduledBackup{}, &NebulaScheduledBackupList{})
}
