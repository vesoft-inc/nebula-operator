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

package nodezone

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodeZone is a plugin that checks node zone.
type NodeZone struct {
	handler    framework.Handle
	cmLister   corelisters.ConfigMapLister
	podLister  corelisters.PodLister
	nodeLister corelisters.NodeLister
}

var _ framework.PreFilterPlugin = &NodeZone{}
var _ framework.FilterPlugin = &NodeZone{}
var _ framework.PermitPlugin = &NodeZone{}
var _ framework.ReservePlugin = &NodeZone{}
var _ framework.EnqueueExtensions = &NodeZone{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "NodeZone"

	// AvailableZones is the number failure domains over which we should spread.
	AvailableZones = 3

	// WaitTime is the max wait time in Permit Stage.
	WaitTime = time.Second * 10

	// preFilterStateKey is the key in CycleState to NodeZone pre-computed data.
	// Using the name of the plugin will likely help us avoid collisions with other plugins.
	preFilterStateKey framework.StateKey = "PreFilter" + Name

	// ErrReasonNoLabelTopologyZone is used for predicate error.
	ErrReasonNoLabelTopologyZone = "node(s) no topology zone label"

	// ErrReasonNotMatch returned when node topology zone doesn't match.
	ErrReasonNotMatch = "node(s) didn't match the requested topology zone"
)

type preFilterState map[string]string

// Clone the prefilter state.
func (s preFilterState) Clone() framework.StateData {
	// The state is not impacted by adding/removing existing pods, hence we don't need to make a deep copy.
	return s
}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *NodeZone) Name() string {
	return Name
}

func (pl *NodeZone) getZoneMappingData(pod *corev1.Pod) (map[string]string, *framework.Status) {
	cmName := getConfigMapName(pod)
	if cmName == "" {
		return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, "ConfigMap name is empty")
	}
	cm, err := pl.cmLister.ConfigMaps(pod.Namespace).Get(cmName)
	if s := getErrorAsStatus(err); !s.IsSuccess() {
		return nil, s
	}
	return cm.Data, nil
}

func getErrorAsStatus(err error) *framework.Status {
	if err != nil {
		if apierrors.IsNotFound(err) {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
		}
		return framework.AsStatus(err)
	}
	return nil
}

// PreFilter invoked at the prefilter extension point.
func (pl *NodeZone) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	// Skip if a pod has no topology spread constraints.
	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return nil, framework.NewStatus(framework.Skip)
	}
	if !needSchedule(pod.Name) {
		return nil, framework.NewStatus(framework.Skip)
	}
	data, status := pl.getZoneMappingData(pod)
	if !status.IsSuccess() {
		return nil, status
	}
	if len(data) == 0 {
		return nil, nil
	}
	cycleState.Write(preFilterStateKey, preFilterState(data))
	return nil, nil
}

// PreFilterExtensions do not exist for this plugin.
func (pl *NodeZone) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func getPreFilterState(cycleState *framework.CycleState) (preFilterState, error) {
	c, err := cycleState.Read(preFilterStateKey)
	if err != nil {
		// preFilterState doesn't exist, likely PreFilter wasn't invoked.
		return nil, fmt.Errorf("reading %q from cycleState: %w", preFilterStateKey, err)
	}

	s, ok := c.(preFilterState)
	if !ok {
		return nil, fmt.Errorf("%+v  convert to nodezone.preFilterState error", c)
	}
	return s, nil
}

func (pl *NodeZone) getPod(podName, namespace string) (*corev1.Pod, error) {
	pod, err := pl.podLister.Pods(namespace).Get(podName)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func (pl *NodeZone) getNode(nodeName string) (*corev1.Node, error) {
	node, err := pl.nodeLister.Get(nodeName)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// getTopologyZones find all zones that represents a logical failure domain.
func (pl *NodeZone) getTopologyZones() ([]string, error) {
	nodeInfo, err := pl.handler.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return nil, fmt.Errorf("listing NodeInfos: %v", err)
	}

	zones := sets.New[string]()
	for _, node := range nodeInfo {
		if len(zones) == AvailableZones {
			break
		}
		nodeZone, ok := node.Node().GetLabels()[corev1.LabelTopologyZone]
		if !ok {
			continue
		}
		zones.Insert(nodeZone)
	}
	sortedZones := sets.List(zones)
	return sortedZones, nil
}

// Filter invoked at the filter extension point.
func (pl *NodeZone) Filter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	state, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	nodeZone, ok := nodeInfo.Node().GetLabels()[corev1.LabelTopologyZone]
	if !ok {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNoLabelTopologyZone)
	}

	stateZone := state[pod.Name]
	if stateZone != "" {
		if stateZone != nodeZone {
			klog.V(5).Infof("pod [%s/%s] not fit node %s due to zone mapping exists", pod.Namespace, pod.Name, nodeInfo.Node().Name)
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNotMatch)
		}
		return nil
	}

	zones, err := pl.getTopologyZones()
	if err != nil {
		return framework.AsStatus(err)
	}
	zoneIndex := make(map[string]int)
	for index, zone := range zones {
		zoneIndex[zone] = index
	}

	klog.V(5).Infof("Available topology zones: %v", zones)

	parentName, ordinal := getParentNameAndOrdinal(pod)
	remainder := ordinal % AvailableZones
	if remainder == 0 {
		return nil
	}

	anchorOrdinal := ordinal - remainder
	anchorName := getPodNameByOrdinal(parentName, anchorOrdinal)
	anchorPod, err := pl.getPod(anchorName, pod.Namespace)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return framework.AsStatus(err)
		}
	}

	if isPodScheduled(anchorPod) {
		anchorNode, err := pl.getNode(anchorPod.Spec.NodeName)
		if s := getErrorAsStatus(err); !s.IsSuccess() {
			return s
		}
		anchorZone, ok := anchorNode.GetLabels()[corev1.LabelTopologyZone]
		if !ok {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNoLabelTopologyZone)
		}
		shift := zoneIndex[anchorZone]
		klog.V(5).Infof("Anchor pod %s zone %s shift %d", anchorName, anchorZone, shift)
		idealZone := zones[(shift+remainder)%AvailableZones]
		if idealZone != nodeZone {
			klog.V(5).Infof("Pod [%s/%s] fit node %s in zone %s, ideal zone %s", pod.Namespace, pod.Name, nodeInfo.Node().Name, nodeZone, idealZone)
		}
		if anchorZone == nodeZone {
			klog.V(5).Infof("Anchor pod [%s/%s] exists in zone %s", pod.Namespace, anchorPod.Name, anchorZone)
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNotMatch)
		}
	}

	var siblingOrdinal int
	if remainder == 1 {
		siblingOrdinal = ordinal + 1
	} else if remainder == 2 {
		siblingOrdinal = ordinal - 1
	}
	siblingName := getPodNameByOrdinal(parentName, siblingOrdinal)
	siblingPod, err := pl.getPod(siblingName, pod.Namespace)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return framework.AsStatus(err)
		}
	}
	if isPodScheduled(siblingPod) {
		siblingNode, err := pl.getNode(siblingPod.Spec.NodeName)
		if s := getErrorAsStatus(err); !s.IsSuccess() {
			return s
		}
		siblingZone, ok := siblingNode.GetLabels()[corev1.LabelTopologyZone]
		if !ok {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNoLabelTopologyZone)
		}
		if siblingZone == nodeZone {
			klog.V(5).Infof("Sibling pod [%s/%s] exists in zone %s", pod.Namespace, siblingPod.Name, siblingZone)
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNotMatch)
		}
	}

	return nil
}

// Permit is the functions invoked by the framework at "Permit" extension point.
func (pl *NodeZone) Permit(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeName string) (*framework.Status, time.Duration) {
	if !needSchedule(pod.Name) {
		return framework.NewStatus(framework.Success), 0
	}

	parentName, ordinal := getParentNameAndOrdinal(pod)
	remainder := ordinal % AvailableZones
	if remainder == 0 {
		return framework.NewStatus(framework.Success), 0
	}

	anchorOrdinal := ordinal - remainder
	anchorName := getPodNameByOrdinal(parentName, anchorOrdinal)
	anchorPod, err := pl.getPod(anchorName, pod.Namespace)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return framework.AsStatus(err), 0
		}
	}
	if !isPodScheduled(anchorPod) {
		klog.InfoS("Anchor pod is waiting to be scheduled to node", "pod", anchorName, "namespace", pod.Namespace, "nodeName", nodeName)
		return framework.NewStatus(framework.Wait), WaitTime
	}

	assumedNode, err := pl.getNode(nodeName)
	if s := getErrorAsStatus(err); !s.IsSuccess() {
		return s, 0
	}
	assumedZone := assumedNode.GetLabels()[corev1.LabelTopologyZone]

	anchorNode, err := pl.getNode(anchorPod.Spec.NodeName)
	if s := getErrorAsStatus(err); !s.IsSuccess() {
		return s, 0
	}
	anchorZone := anchorNode.GetLabels()[corev1.LabelTopologyZone]
	if anchorZone == assumedZone {
		klog.V(5).Infof("Zone conflict, anchor pod [%s/%s] exists in zone %s", pod.Namespace, anchorName, anchorZone)
		return framework.NewStatus(framework.Unschedulable, ErrReasonNotMatch), 0
	}

	if remainder == 2 {
		siblingOrdinal := ordinal - 1
		siblingName := getPodNameByOrdinal(parentName, siblingOrdinal)
		siblingPod, err := pl.getPod(siblingName, pod.Namespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return framework.AsStatus(err), 0
			}
		}
		if !isPodScheduled(siblingPod) {
			klog.InfoS("Sibling pod is waiting to be scheduled to node", "pod", siblingName, "namespace", pod.Namespace, "nodeName", nodeName)
			return framework.NewStatus(framework.Wait), WaitTime
		}
		siblingNode, err := pl.getNode(siblingPod.Spec.NodeName)
		if s := getErrorAsStatus(err); !s.IsSuccess() {
			return s, 0
		}
		siblingZone, ok := siblingNode.GetLabels()[corev1.LabelTopologyZone]
		if !ok {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNoLabelTopologyZone), 0
		}
		if siblingZone == assumedZone {
			klog.V(5).Infof("Zone conflict, sibling pod [%s/%s] exists in zone %s", pod.Namespace, siblingName, siblingZone)
			return framework.NewStatus(framework.Unschedulable, ErrReasonNotMatch), 0
		}
	}

	klog.V(3).InfoS("Permit allows", "pod", klog.KObj(pod))
	return framework.NewStatus(framework.Success), 0
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (pl *NodeZone) Reserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	return nil
}

// Unreserve rejects all other adjacent Pods times out.
func (pl *NodeZone) Unreserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) {
	parentName, ordinal := getParentNameAndOrdinal(pod)
	remainder := ordinal % AvailableZones
	if remainder == 0 {
		return
	}

	quotient := ordinal / AvailableZones
	pl.handler.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		wParentName, wOrdinal := getParentNameAndOrdinal(waitingPod.GetPod())
		wQuotient := wOrdinal / AvailableZones
		if waitingPod.GetPod().Namespace == pod.Namespace && wParentName == parentName && quotient == wQuotient {
			klog.V(3).InfoS("Unreserve rejects", "pod", klog.KObj(waitingPod.GetPod()))
			waitingPod.Reject(pl.Name(), "rejection in Unreserve")
		}
	})
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
func (pl *NodeZone) EventsToRegister() []framework.ClusterEventWithHint {
	return []framework.ClusterEventWithHint{
		{Event: framework.ClusterEvent{Resource: framework.Pod, ActionType: framework.All}},
		{Event: framework.ClusterEvent{Resource: framework.Node, ActionType: framework.All}},
	}
}

// New initializes a new plugin and returns it.
func New(_ context.Context, _ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	informerFactory := handle.SharedInformerFactory()
	cmLister := informerFactory.Core().V1().ConfigMaps().Lister()
	podLister := informerFactory.Core().V1().Pods().Lister()
	nodeLister := informerFactory.Core().V1().Nodes().Lister()
	return &NodeZone{
		handle,
		cmLister,
		podLister,
		nodeLister,
	}, nil
}
