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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="nb"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`,description="The current status of the backup"
// +kubebuilder:printcolumn:name="Started",type=date,JSONPath=`.status.timeStarted`,description="The time at which the backup was started"
// +kubebuilder:printcolumn:name="Completed",type=date,JSONPath=`.status.timeCompleted`,description="The time at which the backup was completed"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type NebulaBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// NebulaBackupList contains a list of NebulaBackup.
type NebulaBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NebulaBackup `json:"items"`
}

// BackupConditionType represents a valid condition of a Backup.
type BackupConditionType string

const (
	// BackupPending means the backup is pending, waiting for create backup job
	BackupPending BackupConditionType = "Pending"
	// BackupRunning means the backup is running.
	BackupRunning BackupConditionType = "Running"
	// BackupComplete means the backup has successfully executed and the
	// backup data has been loaded into nebula cluster.
	BackupComplete BackupConditionType = "Complete"
	// BackupFailed means the backup has failed.
	BackupFailed BackupConditionType = "Failed"
	// BackupInvalid means invalid backup CR.
	BackupInvalid BackupConditionType = "Invalid"
)

// BackupCondition describes the observed state of a Backup at a certain point.
type BackupCondition struct {
	// Type of the condition.
	Type BackupConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// BackupSpec contains the specification for a backup of a nebula cluster backup.
type BackupSpec struct {
	// toolImage
	ToolImage string `json:"toolImage,omitempty"`

	// +kubebuilder:default=Always
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	BR *BRConfig `json:"br,omitempty"`

	// MaxBackups is to specify how many backups we want to keep. Used for scheduled backups only.
	MaxBackups *int32 `json:"maxBackups,omitempty"`

	// MaxReservedTime is to specify how long backups we want to keep. Used for scheduled backups only.
	MaxReservedTime *string `json:"maxReservedTime,omitempty"`
}

// BackupStatus represents the current status of a nebula cluster backup.
type BackupStatus struct {
	// TimeStarted is the time at which the backup was started.
	// +nullable
	TimeStarted metav1.Time `json:"timeStarted,omitempty"`
	// TimeCompleted is the time at which the backup was completed.
	// +nullable
	TimeCompleted metav1.Time `json:"timeCompleted,omitempty"`
	// Phase is a user readable state inferred from the underlying Backup conditions
	Phase BackupConditionType `json:"phase,omitempty"`
	// +nullable
	Conditions []BackupCondition `json:"conditions,omitempty"`

	// BackupName is the dir name of the backup
	BackupName string `json:"backupName,omitempty"`

	// JobName is the name of the backup job
	JobName string `json:"jobName,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NebulaBackup{}, &NebulaBackupList{})
}
