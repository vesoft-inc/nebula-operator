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

	"github.com/vesoft-inc/nebula-go/v3/nebula"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="nr"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`,description="The current status of the restore"
// +kubebuilder:printcolumn:name="RESTORED-CLUSTER",type=string,JSONPath=`.status.clusterName`,description="The name of restored nebula cluster"
// +kubebuilder:printcolumn:name="Started",type=date,JSONPath=`.status.timeStarted`,description="The time at which the restore was started"
// +kubebuilder:printcolumn:name="Completed",type=date,JSONPath=`.status.timeCompleted`,description="The time at which the restore was completed"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type NebulaRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NebulaRestoreList contains a list of NebulaRestore.
type NebulaRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NebulaRestore `json:"items"`
}

// RestoreConditionType represents a valid condition of a Restore.
type RestoreConditionType string

const (
	// RestoreComplete means the restore has successfully executed and the
	// backup data has been loaded into nebula cluster.
	RestoreComplete RestoreConditionType = "Complete"
	// RestoreMetadComplete means metad pods have been rebuilt from the backup data
	RestoreMetadComplete RestoreConditionType = "MetadComplete"
	// RestoreStoragedCompleted means storaged pods have been rebuilt from the backup data
	RestoreStoragedCompleted RestoreConditionType = "StoragedComplete"
	// RestoreFailed means the restore has failed.
	RestoreFailed RestoreConditionType = "Failed"
	// RestoreInvalid means invalid restore CR.
	RestoreInvalid RestoreConditionType = "Invalid"
)

// RestoreCondition describes the observed state of a Restore at a certain point.
type RestoreCondition struct {
	// Type of the condition.
	Type RestoreConditionType `json:"type"`
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

type RestoreConfig struct {
	NamespacedObjectReference `json:",inline"`
	// The name of the backup file.
	BackupName string `json:"backupName,omitempty"`
	// Concurrency is used to control the number of concurrent file downloads during data restoration.
	// +kubebuilder:default=5
	// +optional
	Concurrency int32 `json:"concurrency,omitempty"`
	// StorageProvider configures where and how backups should be stored.
	StorageProvider `json:",inline"`
}

// RestoreSpec contains the specification for a restore of a nebula cluster backup.
type RestoreSpec struct {
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// +optional
	AutoRemoveFailed bool `json:"autoRemoveFailed,omitempty"`

	// +optional
	Config *RestoreConfig `json:"config,omitempty"`
}

// RestoreStatus represents the current status of a nebula cluster restore.
type RestoreStatus struct {
	// TimeStarted is the time at which the restore was started.
	// +nullable
	TimeStarted *metav1.Time `json:"timeStarted,omitempty"`
	// TimeCompleted is the time at which the restore was completed.
	// +nullable
	TimeCompleted *metav1.Time `json:"timeCompleted,omitempty"`
	// ClusterName is the name of restored nebula cluster.
	ClusterName string `json:"clusterName,omitempty"`
	// Phase is a user readable state inferred from the underlying Restore conditions
	Phase RestoreConditionType `json:"phase,omitempty"`
	// +nullable
	Conditions []RestoreCondition `json:"conditions,omitempty"`
	// +nullable
	Partitions map[string][]*nebula.HostAddr `json:"partitions,omitempty"`
	// +nullable
	Checkpoints map[string]map[string]string `json:"checkpoints,omitempty"`
}

func init() {
	SchemeBuilder.Register(&NebulaRestore{}, &NebulaRestoreList{})
}
