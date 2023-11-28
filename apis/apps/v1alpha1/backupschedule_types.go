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

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="bs"

type BackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupScheduleSpec   `json:"spec,omitempty"`
	Status BackupScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// BackupScheduleList contains a list of BackupSchedule.
type BackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []BackupSchedule `json:"items"`
}

// BackupScheduleSpec contains the specification for a backupSchedule of a nebula cluster backupSchedule.
type BackupScheduleSpec struct {
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

// BackupScheduleStatus represents the current status of a nebula cluster backupSchedule.
type BackupScheduleStatus struct {
	// LastBackup represents the last backup.
	LastBackup string `json:"lastBackup,omitempty"`
	// LastBackupTime represents the last time the backup was successfully created.
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&BackupSchedule{}, &BackupScheduleList{})
}
