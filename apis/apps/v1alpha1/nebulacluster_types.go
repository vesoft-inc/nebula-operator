/*
Copyright 2021 Vesoft Inc.

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

// NebulaClusterConditionType represents a nebula cluster condition value.
type NebulaClusterConditionType string

const (
	// NebulaClusterReady indicates that the nebula cluster is ready or not.
	// This is defined as:
	// - All workloads are up to date (currentRevision == updateRevision).
	// - All nebula component pods are healthy.
	NebulaClusterReady NebulaClusterConditionType = "Ready"
)

// ComponentPhase is the current state of component
type ComponentPhase string

const (
	// RunningPhase represents normal state of nebula cluster.
	RunningPhase ComponentPhase = "Running"
	// UpgradePhase represents the upgrade state of nebula cluster.
	UpgradePhase ComponentPhase = "Upgrade"
	// ScaleInPhase represents the scaling state of nebula cluster.
	ScaleInPhase ComponentPhase = "ScaleIn"
	// ScaleOutPhase represents the scaling state of nebula cluster.
	ScaleOutPhase ComponentPhase = "ScaleOut"
	// UpdatePhase represents update state of nebula cluster.
	UpdatePhase ComponentPhase = "Update"
)

// NebulaClusterSpec defines the desired state of NebulaCluster
type NebulaClusterSpec struct {
	// graphd spec
	Graphd *GraphdSpec `json:"graphd"`

	// Metad spec
	Metad *MetadSpec `json:"metad"`

	// Storaged spec
	Storaged *StoragedSpec `json:"storaged"`

	// +optional
	Reference WorkloadReference `json:"reference,omitempty"`

	// +kubebuilder:default=default-scheduler
	// +optional
	SchedulerName string `json:"schedulerName"`

	// +kubebuilder:default=DoNotSchedule
	// +optional
	UnsatisfiableAction corev1.UnsatisfiableConstraintAction `json:"unsatisfiableAction"`

	// Flag to enable/disable pv reclaim while the nebula cluster deleted , default false
	// +optional
	EnablePVReclaim *bool `json:"enablePVReclaim,omitempty"`

	// +kubebuilder:default=Always
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"toleration,omitempty"`

	// UpdatePolicy indicates how pods should be updated
	// +optional
	UpdatePolicy string `json:"strategy,omitempty"`

	// Flag to enable/disable sidecar container nebula-agent injection, default false.
	// +optional
	EnableBR *bool `json:"enableBR,omitempty"`

	// +optional
	LogRotate *LogRotate `json:"logRotate,omitempty"`

	// +optional
	Exporter *ExporterSpec `json:"exporter,omitempty"`

	// SSLCerts defines SSL certs load into secret
	SSLCerts *SSLCertsSpec `json:"sslCerts,omitempty"`
}

// NebulaClusterStatus defines the observed state of NebulaCluster
type NebulaClusterStatus struct {
	Graphd     ComponentStatus          `json:"graphd,omitempty"`
	Metad      ComponentStatus          `json:"metad,omitempty"`
	Storaged   StoragedStatus           `json:"storaged,omitempty"`
	Conditions []NebulaClusterCondition `json:"conditions,omitempty"`
	Version    string                   `json:"version,omitempty"`
}

// ComponentStatus is the status and version of a nebula component.
type ComponentStatus struct {
	Version  string         `json:"version,omitempty"`
	Phase    ComponentPhase `json:"phase,omitempty"`
	Workload WorkloadStatus `json:"workload,omitempty"`
}

// StoragedStatus describes the status and version of nebula storaged.
type StoragedStatus struct {
	ComponentStatus `json:",inline"`
	HostsAdded      bool        `json:"hostsAdded,omitempty"`
	BalancedSpaces  []int32     `json:"balancedSpaces,omitempty"`
	LastBalanceJob  *BalanceJob `json:"lastBalanceJob,omitempty"`
}

// BalanceJob describes the admin job for balance data.
type BalanceJob struct {
	SpaceID int32 `json:"spaceID,omitempty"`
	JobID   int32 `json:"jobID,omitempty"`
}

// WorkloadStatus describes the status of a specified workload.
type WorkloadStatus struct {
	// ObservedGeneration is the most recent generation observed for this Workload. It corresponds to the
	// Workload's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The number of ready replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Replicas is the most recently observed number of replicas.
	Replicas int32 `json:"replicas"`

	// The number of pods in current version.
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// The number of ready current revision replicas for this Workload.
	// +optional
	UpdatedReadyReplicas int32 `json:"updatedReadyReplicas,omitempty"`

	// Count of hash collisions for the Workload.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`

	// CurrentRevision, if not empty, indicates the current version of the Workload.
	CurrentRevision string `json:"currentRevision"`

	// updateRevision, if not empty, indicates the version of the Workload used to generate Pods in the sequence
	UpdateRevision string `json:"updateRevision,omitempty"`
}

// NebulaClusterCondition describes the state of a nebula cluster at a certain point.
type NebulaClusterCondition struct {
	// Type of the condition.
	Type NebulaClusterConditionType `json:"type"`
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

// A WorkloadReference refers to a CustomResourceDefinition by name.
type WorkloadReference struct {
	// Name of the referenced CustomResourceDefinition.
	// eg. statefulsets.apps
	Name string `json:"name"`

	// Version indicate which version should be used if CRD has multiple versions
	// by default it will use the first one if not specified
	Version string `json:"version,omitempty"`
}

type LogRotate struct {
	// +kubebuilder:default=5
	// +optional
	Rotate int32 `json:"rotate,omitempty"`

	// +kubebuilder:default="200M"
	// +optional
	Size string `json:"size,omitempty"`
}

// ExporterSpec defines the desired state of Exporter
type ExporterSpec struct {
	PodSpec `json:",inline"`

	// Maximum number of parallel scrape requests
	// +kubebuilder:default=40
	// +optional
	MaxRequests int32 `json:"maxRequests,omitempty"`
}

type LicenseSpec struct {
	// Name of the license secret.
	SecretName string `json:"secretName,omitempty"`
	// The key to nebula license file.
	LicenseKey string `json:"licenseKey,omitempty"`
}

type SSLCertsSpec struct {
	// Name of the server cert secret
	ServerSecret string `json:"serverSecret,omitempty"`
	// The key to server PEM encoded public key certificate
	// +kubebuilder:default=tls.crt
	// +optional
	ServerPublicKey string `json:"serverPublicKey,omitempty"`
	// The key to server private key associated with given certificate
	// +kubebuilder:default=tls.key
	// +optional
	ServerPrivateKey string `json:"serverPrivateKey,omitempty"`

	// Name of the client cert secret
	ClientSecret string `json:"clientSecret,omitempty"`
	// The key to client PEM encoded public key certificate
	// +kubebuilder:default=tls.crt
	// +optional
	ClientPublicKey string `json:"clientPublicKey,omitempty"`
	// The key to client private key associated with given certificate
	// +kubebuilder:default=tls.key
	// +optional
	ClientPrivateKey string `json:"clientPrivateKey,omitempty"`

	// Name of the CA cert secret
	CASecret string `json:"caSecret,omitempty"`
	// The key to CA PEM encoded public key certificate
	// +kubebuilder:default=ca.crt
	// +optional
	CAPublicKey string `json:"caPublicKey,omitempty"`

	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name.
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`
}

// GraphdSpec defines the desired state of Graphd
type GraphdSpec struct {
	PodSpec `json:",inline"`

	// Config defines a graphd configuration load into ConfigMap
	Config map[string]string `json:"config,omitempty"`

	// Service defines a k8s service of Graphd cluster.
	// +optional
	Service *GraphdServiceSpec `json:"service,omitempty"`

	// K8S persistent volume claim for Graphd log volume.
	// +optional
	LogVolumeClaim *StorageClaim `json:"logVolumeClaim,omitempty"`
}

// MetadSpec defines the desired state of Metad
type MetadSpec struct {
	PodSpec `json:",inline"`

	// Config defines a metad configuration load into ConfigMap
	Config map[string]string `json:"config,omitempty"`

	// Service defines a Kubernetes service of Metad cluster.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// K8S persistent volume claim for Metad log volume.
	// +optional
	LogVolumeClaim *StorageClaim `json:"logVolumeClaim,omitempty"`

	// K8S persistent volume claim for Metad data volume.
	// +optional
	DataVolumeClaim *StorageClaim `json:"dataVolumeClaim,omitempty"`

	// License defines a nebula license load into Secret
	License *LicenseSpec `json:"license,omitempty"`
}

// StoragedSpec defines the desired state of Storaged
type StoragedSpec struct {
	PodSpec `json:",inline"`

	// Config defines a storaged configuration load into ConfigMap
	Config map[string]string `json:"config,omitempty"`

	// Service defines a Kubernetes service of Storaged cluster.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// K8S persistent volume claim for Storaged log volume.
	// +optional
	LogVolumeClaim *StorageClaim `json:"logVolumeClaim,omitempty"`

	// K8S persistent volume claim for Storaged data volume.
	// +optional
	DataVolumeClaims []StorageClaim `json:"dataVolumeClaims,omitempty"`

	// Flag to enable/disable auto balance data and leader while the nebula storaged scale out, default false
	// +optional
	EnableAutoBalance *bool `json:"enableAutoBalance,omitempty"`

	// Flag to enable/disable rolling update without leader state transition
	// +optional
	EnableForceUpdate *bool `json:"enableForceUpdate,omitempty"`
}

// PodSpec is a common set of k8s resource configs for nebula components.
type PodSpec struct {
	// K8S deployment replicas setting.
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// K8S resources settings.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Container environment variables.
	// +optional
	EnvVars []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`

	// Version tag for docker images
	// +optional
	Version string `json:"version,omitempty"`

	// K8S pod annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// K8S nodeSelector.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// +optional
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// +optional
	SidecarVolumes []corev1.Volume `json:"sidecarVolumes,omitempty"`

	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
}

// StorageClaim contains details of storage
type StorageClaim struct {
	// Resources represents the minimum resources the volume should have.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Name of the StorageClass required by the claim.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// GraphdServiceSpec is the service spec of graphd
type GraphdServiceSpec struct {
	ServiceSpec `json:",inline"`

	// LoadBalancerIP is the loadBalancerIP of service
	// +optional
	LoadBalancerIP *string `json:"loadBalancerIP,omitempty"`

	// ExternalTrafficPolicy of the service
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
}

// ServiceSpec is a common set of k8s service configs.
type ServiceSpec struct {
	Type corev1.ServiceType `json:"type,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Selector map[string]string `json:"selector,omitempty"`

	// ClusterIP is the clusterIP of service
	// +optional
	ClusterIP *string `json:"clusterIP,omitempty"`

	// +optional
	PublishNotReadyAddresses bool `json:"publishNotReadyAddresses,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nc
// +kubebuilder:printcolumn:name="GRAPHD-DESIRED",type="string",JSONPath=".spec.graphd.replicas",description="The desired number of graphd pods."
// +kubebuilder:printcolumn:name="GRAPHD-READY",type="string",JSONPath=".status.graphd.workload.readyReplicas",description="The number of graphd pods ready."
// +kubebuilder:printcolumn:name="METAD-DESIRED",type="string",JSONPath=".spec.metad.replicas",description="The desired number of metad pods."
// +kubebuilder:printcolumn:name="METAD-READY",type="string",JSONPath=".status.metad.workload.readyReplicas",description="The number of metad pods ready."
// +kubebuilder:printcolumn:name="STORAGED-DESIRED",type="string",JSONPath=".spec.storaged.replicas",description="The desired number of storaged pods."
// +kubebuilder:printcolumn:name="STORAGED-READY",type="string",JSONPath=".status.storaged.workload.readyReplicas",description="The number of storaged pods ready."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created. It is represented in RFC3339 form and is in UTC."

// NebulaCluster is the Schema for the nebulaclusters API
type NebulaCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NebulaClusterSpec   `json:"spec,omitempty"`
	Status NebulaClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NebulaClusterList contains a list of NebulaCluster
type NebulaClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NebulaCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NebulaCluster{}, &NebulaClusterList{})
}
