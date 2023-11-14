package v1alpha1

import (
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// DefaultPollingPeriod is the default period between 2 autoscaling API polls.
	DefaultPollingPeriod = 30 * time.Second
)

type NebulaAutoscalerConditionType string

const (
	// AutoscalerActive indicates that the HPA controller is able to scale if necessary:
	// it's correctly configured, can fetch the desired metrics, and isn't disabled.
	AutoscalerActive NebulaAutoscalerConditionType = "Active"

	// AbleToScale indicates a lack of transient issues which prevent scaling from occurring,
	// such as being in a backoff window, or being unable to access/update the target scale.
	AbleToScale NebulaAutoscalerConditionType = "AbleToScale"

	// AutoscalerLimited indicates that the calculated scale based on metrics would be above or
	// below the range for the HPA, and has thus been capped.
	AutoscalerLimited NebulaAutoscalerConditionType = "Limited"

	// AutoscalerReady indicates that the nebula cluster is ready.
	AutoscalerReady NebulaAutoscalerConditionType = "Ready"
)

// NebulaAutoscalerSpec defines the desired state of NebulaAutoscaler
type NebulaAutoscalerSpec struct {
	// +kubebuilder:validation:Required
	NebulaClusterRef NebulaClusterRef `json:"nebulaClusterRef"`

	GraphdPolicy AutoscalingPolicySpec `json:"graphdPolicy"`

	// +optional
	// PollingPeriod is the period at which to synchronize with the autoscaling API.
	PollingPeriod *metav1.Duration `json:"pollingPeriod,omitempty"`
}

type NebulaClusterRef struct {
	// Name is the name of the NebulaCluster resource to scale automatically.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type AutoscalingPolicySpec struct {
	// MinReplicas is the lower limit for the number of replicas to which the autoscaler
	// can scale down.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas"`

	// Metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +optional
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`

	// Behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default HPAScalingRules for scale up and scale down are used.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

type NebulaAutoscalerCondition struct {
	// Type describes the current condition
	Type NebulaAutoscalerConditionType `json:"type"`

	// Status is the status of the condition (True, False, Unknown)
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned from
	// one status to another
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is the reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation containing details about
	// the transition
	// +optional
	Message string `json:"message,omitempty"`
}

// NebulaAutoscalerStatus defines the observed state of NebulaAutoscaler
type NebulaAutoscalerStatus struct {
	// ObservedGeneration is the most recent generation observed by this autoscaler.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// GraphdStatuses describes the status of Graphd autoscaling policies.
	// +optional
	GraphdStatus AutoscalingPolicyStatus `json:"graphdStatus,omitempty"`

	// Conditions is the set of conditions required for this autoscaler to scale its target,
	// and indicates whether those conditions are met.
	// +optional
	Conditions []NebulaAutoscalerCondition `json:"conditions,omitempty"`
}

type AutoscalingPolicyStatus struct {
	// LastScaleTime is the last time the autoscaler scaled the number of pods,
	// used by the autoscaler to control how often the number of pods is changed.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// CurrentReplicas is current number of replicas of pods managed by this autoscaler,
	// as last seen by the autoscaler.
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas of pods managed by this autoscaler,
	// as last calculated by the autoscaler.
	DesiredReplicas int32 `json:"desiredReplicas"`

	// CurrentMetrics is the last read state of the metrics used by this autoscaler.
	// +optional
	CurrentMetrics []autoscalingv2.MetricStatus `json:"currentMetrics"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=na
// +kubebuilder:printcolumn:name="REFERENCE",type="string",JSONPath=".spec.nebulaClusterRef.name"
// +kubebuilder:printcolumn:name="MIN-REPLICAS",type="string",JSONPath=".spec.graphdPolicy.minReplicas"
// +kubebuilder:printcolumn:name="MAX-REPLICAS",type="string",JSONPath=".spec.graphdPolicy.maxReplicas"
// +kubebuilder:printcolumn:name="CURRENT-REPLICAS",type="string",JSONPath=".status.graphdStatus.currentReplicas"
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type=='Active')].status"
// +kubebuilder:printcolumn:name="AbleToScale",type="string",JSONPath=".status.conditions[?(@.type=='AbleToScale')].status"
// +kubebuilder:printcolumn:name="Limited",type="string",JSONPath=".status.conditions[?(@.type=='Limited')].status"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created. It is represented in RFC3339 form and is in UTC."

// NebulaAutoscaler represents an NebulaAutoscaler resource in a Kubernetes cluster
type NebulaAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NebulaAutoscalerSpec   `json:"spec,omitempty"`
	Status NebulaAutoscalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NebulaAutoscalerList contains a list of NebulaAutoscaler
type NebulaAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NebulaAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NebulaAutoscaler{}, &NebulaAutoscalerList{})
}
