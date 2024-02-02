/*
Copyright 2024 Vesoft Inc.

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
// +kubebuilder:resource:shortName="ncb"
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`,description="The cron format string used for backup scheduling"
// +kubebuilder:printcolumn:name="LastBackup",type=string,JSONPath=`.status.lastBackup`,description="The last backup CR name"
// +kubebuilder:printcolumn:name="LastScheduleTime",type=date,JSONPath=`.status.lastScheduleTime`,description="The last time the backup was successfully scheduled"
// +kubebuilder:printcolumn:name="LastSuccessfulTime",type=date,JSONPath=`.status.lastSuccessfulTime`,description="The last time the backup successfully completed"
// +kubebuilder:printcolumn:name="BackupCleanTime",type=date,JSONPath=`.status.backupCleanTime`,description="The last time the expired backup objects are cleaned up"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type NebulaCronBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronBackupSpec   `json:"spec,omitempty"`
	Status CronBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NebulaCronBackupList contains a list of NebulaCronBackup.
type NebulaCronBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NebulaCronBackup `json:"items"`
}

type CronBackupSpec struct {
	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	Schedule string `json:"schedule"`

	// This flag tells the controller to pause subsequent executions, it does
	// not apply to already started executions. Defaults to false.
	// +optional
	Pause *bool `json:"pause,omitempty"`

	// MaxReservedTime is to specify how long backups we want to keep.
	// It should be a duration string format.
	// +optional
	MaxReservedTime *string `json:"maxReservedTime,omitempty"`

	// Specifies the backup that will be created when executing a CronBackup.
	BackupTemplate BackupSpec `json:"backupTemplate"`
}

type CronBackupStatus struct {
	// LastBackup represents the last backup.
	LastBackup string `json:"lastBackup,omitempty"`
	// LastScheduleTime represents the last time the backup was successfully scheduled.
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// LastSuccessfulTime represents the last time the backup successfully completed.
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
	// BackupCleanTime represents the last time the expired backup objects are cleaned up.
	BackupCleanTime *metav1.Time `json:"backupCleanTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NebulaCronBackup{}, &NebulaCronBackupList{})
}
