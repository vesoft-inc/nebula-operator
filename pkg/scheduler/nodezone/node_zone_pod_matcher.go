package nodezone

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
)

// NodeInfo is a label snapshot of a node stored in ZoneCapacityTracker.
type NodeInfo struct {
	name   string
	zone   string
	labels map[string]string
}

// podMatcher evaluates whether a node satisfies a pod's nodeSelector AND
// nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.
// It is built once per scheduling cycle in PreFilter and shared with Filter.
type podMatcher struct {
	// nodeSelector: simple map[string]string equality (all keys must match)
	labelSelector labels.Selector
	// nodeAffinity required terms (nil = match all)
	affinitySelector *nodeaffinity.NodeSelector
}

// newPodMatcher builds a podMatcher from the pod's scheduling constraints.
func newPodMatcher(pod *v1.Pod) (*podMatcher, error) {
	m := &podMatcher{labelSelector: labels.Everything()}

	// --- nodeSelector ---
	if len(pod.Spec.NodeSelector) > 0 {
		m.labelSelector = labels.SelectorFromSet(labels.Set(pod.Spec.NodeSelector))
	}

	// --- nodeAffinity (required terms only) ---
	// PreferredDuringScheduling terms influence scoring but not eligibility,
	// so we intentionally ignore them here.
	if pod.Spec.Affinity != nil &&
		pod.Spec.Affinity.NodeAffinity != nil &&
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {

		// Check if NewNodeSelector is the correct method to use here.
		ns, err := nodeaffinity.NewNodeSelector(
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
		if err != nil {
			return nil, fmt.Errorf("parsing nodeAffinity for pod %q: %w", pod.Name, err)
		}
		m.affinitySelector = ns
	}

	return m, nil
}

// Matches returns true if the node satisfies both nodeSelector and nodeAffinity.
func (m *podMatcher) Matches(info *NodeInfo) bool {
	// nodeSelector check.
	if !m.labelSelector.Matches(labels.Set(info.labels)) {
		return false
	}
	// nodeAffinity check: NodeSelector.Match requires a *v1.Node; build a
	// minimal one from our label snapshot — only ObjectMeta is inspected.
	if m.affinitySelector != nil {
		if !m.affinitySelector.Match(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   info.name,
				Labels: info.labels,
			},
		}) {
			return false
		}
	}
	return true
}

// matchesAll returns true regardless of node labels (used for unscoped checks).
func matchesAll(_ *NodeInfo) bool { return true }
