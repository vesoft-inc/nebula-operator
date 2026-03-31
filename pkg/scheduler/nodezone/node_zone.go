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

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodeZone is a plugin that checks node zone.
type NodeZone struct {
	handler            framework.Handle
	cmLister           corelisters.ConfigMapLister
	podLister          corelisters.PodLister
	nodeLister         corelisters.NodeLister
	trackerGateFactory informers.SharedInformerFactory
	zoneTrackers       map[string]*ZoneCapacityTracker
	serialGates        map[string]*SerialGate
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

	// Constants for serial scheduling and zone tracking.
	stateKeySerial  = "zone-scheduler/serial"
	stateKeyZone    = "zone-scheduler/zone"
	stateKeyMatcher = "zone-scheduler/matcher"

	// Switch to serial when any zone drops to this many available nodes.
	criticalThreshold = 1

	// Only switch back to parallel when all zones are above this threshold
	// (hysteresis to prevent flapping).
	parallelThreshold = 3
)

type preFilterState map[string]string

var (
	componentTypes = []string{
		v1alpha1.GraphdComponentType.String(),
		v1alpha1.StoragedComponentType.String(),
	}
)

// Clone the prefilter state.
func (s preFilterState) Clone() framework.StateData {
	// The state is not impacted by adding/removing existing pods, hence we don't need to make a deep copy.
	return s
}

type serialStateData struct {
	required bool
}

func (s *serialStateData) Clone() framework.StateData { return &serialStateData{required: s.required} }

type zoneStateData struct {
	zone string
}

func (z *zoneStateData) Clone() framework.StateData { return &zoneStateData{zone: z.zone} }

type matcherStateData struct {
	m *podMatcher
}

func (d *matcherStateData) Clone() framework.StateData { return &matcherStateData{m: d.m} }

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
// TODO: upgrade to v1.28.0+
func (pl *NodeZone) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	heading := fmt.Sprintf("Pod [%v/%v]: ", pod.Namespace, pod.Name)
	compType := componentType(pod)

	// Skip if a pod has no topology spread constraints.
	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return nil, nil
	}
	if !needSchedule(pod.Name) {
		return nil, nil
	}
	data, status := pl.getZoneMappingData(pod)
	if !status.IsSuccess() {
		return nil, status
	}
	if len(data) == 0 {
		return nil, nil
	}
	cycleState.Write(preFilterStateKey, preFilterState(data))

	// Build matcher once; Filter reuses it for every candidate node.
	m, err := newPodMatcher(pod)
	if err != nil {
		return nil, framework.AsStatus(err)
	}
	cycleState.Write(stateKeyMatcher, &matcherStateData{m: m})

	pl.zoneTrackers[compType].ZoneStatus(heading, m, pod)
	critical := pl.zoneTrackers[compType].AnyCriticalForPod(m, pod)
	cycleState.Write(stateKeySerial, &serialStateData{required: critical})
	pl.serialGates[compType].SetSerial(critical)

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
	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return nil
	}
	if !needSchedule(pod.Name) {
		return nil
	}

	heading := fmt.Sprintf("Pod [%v/%v]: ", pod.Namespace, pod.Name)

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

	klog.V(5).Infof("%v Available topology zones: %v", heading, zones)

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
		klog.V(5).Infof("%v Anchor pod %s zone %s shift %d", heading, anchorName, anchorZone, shift)
		idealZone := zones[(shift+remainder)%AvailableZones]
		if idealZone != nodeZone {
			klog.V(5).Infof("%v Pod [%s/%s] fit node %s in zone %s, ideal zone %s", heading, pod.Namespace, pod.Name, nodeInfo.Node().Name, nodeZone, idealZone)
		}
		if anchorZone == nodeZone {
			klog.V(5).Infof("%v Anchor pod [%s/%s] exists in zone %s", heading, pod.Namespace, anchorPod.Name, anchorZone)
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
			klog.V(5).Infof("%v Sibling pod [%s/%s] exists in zone %s", heading, pod.Namespace, siblingPod.Name, siblingZone)
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

	klog.Infof("Trying to start pod [%v/%v] on node %v", pod.Namespace, pod.Name, nodeName)
	heading := fmt.Sprintf("Pod [%v/%v]: ", pod.Namespace, pod.Name)

	parentName, ordinal := getParentNameAndOrdinal(pod)

	entry := &PendingPod{
		uid:           pod.UID,
		namespace:     pod.Namespace,
		parent:        parentName,
		number:        ordinal,
		waitingOnZone: false,
		approve: func() {
			pl.handler.IterateOverWaitingPods(func(wp framework.WaitingPod) {
				if wp.GetPod().UID == pod.UID {
					klog.Infof("%v Permit allows serial deployment of pod [%v/%v]", heading, wp.GetPod().Namespace, wp.GetPod().Name)
					wp.Allow(Name)
				}
			})
		},
	}

	compType := componentType(pod)

	remainder := ordinal % AvailableZones

	if remainder != 0 {
		anchorOrdinal := ordinal - remainder
		anchorName := getPodNameByOrdinal(parentName, anchorOrdinal)
		anchorPod, err := pl.getPod(anchorName, pod.Namespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return framework.AsStatus(err), 0
			}
		}
		if !isPodScheduled(anchorPod) {
			klog.InfoS(fmt.Sprintf("%v Anchor pod is waiting to be scheduled to node", heading), "pod", anchorName, "namespace", pod.Namespace, "nodeName", nodeName)
			// Always enqueue — ensures ordering is correct if mode switches
			// to serial while this pod is waiting on its anchor.
			pl.serialGates[compType].Enqueue(entry)
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
			klog.V(5).Infof("%v Zone conflict, anchor pod [%s/%s] exists in zone %s", heading, pod.Namespace, anchorName, anchorZone)
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
				klog.InfoS(fmt.Sprintf("%v Sibling pod is waiting to be scheduled to node", heading), "pod", siblingName, "namespace", pod.Namespace, "nodeName", nodeName)
				// Always enqueue — ensures ordering is correct if mode switches
				// to serial while this pod is waiting on its sibling pod.
				pl.serialGates[compType].Enqueue(entry)
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
				klog.V(5).Infof("%v Zone conflict, sibling pod [%s/%s] exists in zone %s", heading, pod.Namespace, siblingName, siblingZone)
				return framework.NewStatus(framework.Unschedulable, ErrReasonNotMatch), 0
			}
		}
	}

	entry.waitingOnZone = true
	switch pl.serialGates[compType].AcquireOrEnqueue(entry) {
	case AcquiredParallel:
		klog.InfoS(fmt.Sprintf("%v Permit allows parallel deployment of", heading), "pod", klog.KObj(pod))
		// Parallel mode — remove from heap in case it was enqueued earlier
		// during an anchor/sibling wait that has since resolved.
		pl.serialGates[compType].RemoveFromHeap(pod.UID)
		return framework.NewStatus(framework.Success), 0
	case AcquiredSerial:
		klog.InfoS(fmt.Sprintf("%v Permit allows serial deployment of", heading), "pod", klog.KObj(pod))
		return framework.NewStatus(framework.Success), 0
	case Waiting:
		klog.InfoS(fmt.Sprintf("%v Waiting for serial zone gate", heading), "pod", klog.KObj(pod))
		return framework.NewStatus(framework.Wait, "waiting for serial zone gate"), WaitTime
	default:
		return framework.NewStatus(framework.Unschedulable, "Invalid deployment method"), 0
	}
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (pl *NodeZone) Reserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	if !needSchedule(pod.Name) {
		return nil
	}

	nodeInfo, err := pl.handler.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return framework.AsStatus(fmt.Errorf("getting node %q: %w", nodeName, err))
	}

	compType := componentType(pod)

	zone := zoneForNode(nodeInfo.Node())
	state.Write(stateKeyZone, &zoneStateData{zone: zone})
	pl.zoneTrackers[compType].EnsureDecremented(pod, zone)

	return nil
}

// Unreserve rejects all other adjacent Pods times out.
func (pl *NodeZone) Unreserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) {
	if !needSchedule(pod.Name) {
		return
	}

	parentName, ordinal := getParentNameAndOrdinal(pod)
	compType := componentType(pod)

	// Call unconditionally — EnsureIncremented is idempotent and a no-op
	// if this pod was never decremented (uid not in decrementedPods).
	pl.zoneTrackers[compType].EnsureIncremented(pod)
	pl.serialGates[compType].Release(pod)

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
func (pl *NodeZone) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		{Resource: framework.Pod, ActionType: framework.All},
		{Resource: framework.Node, ActionType: framework.All},
	}
}

func (pl *NodeZone) registerNodeInformer(factory informers.SharedInformerFactory) {
	nodeInformer := factory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*v1.Node)
			for _, tracker := range pl.zoneTrackers {
				tracker.AddNode(node)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			newNode := new.(*v1.Node)
			for _, tracker := range pl.zoneTrackers {
				tracker.RemoveNode(old)
				tracker.AddNode(newNode)
			}
		},
		DeleteFunc: func(obj interface{}) {
			node := obj.(*v1.Node)
			for _, tracker := range pl.zoneTrackers {
				tracker.RemoveNode(node)
			}
		},
	})
}

func (pl *NodeZone) registerPodInformer(factory informers.SharedInformerFactory) {
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if !needSchedule(pod.Name) {
				return
			}

			compType := componentType(pod)
			// Covers crash-recovery: scheduler restarted after binding but
			// before Reserve was replayed.
			if isPodScheduled(pod) && !isFinished(pod) {
				zone := pl.zoneForNodeName(pod.Spec.NodeName)
				pl.zoneTrackers[compType].EnsureDecremented(pod, zone)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			oldPod := old.(*v1.Pod)
			newPod := new.(*v1.Pod)
			if !needSchedule(newPod.Name) {
				return
			}

			compType := componentType(newPod)

			// These are independent — all that apply should fire.
			if !isFinished(oldPod) && isFinished(newPod) {
				pl.serialGates[compType].Release(newPod)
				pl.zoneTrackers[compType].EnsureIncremented(newPod)
			}
			if oldPod.Status.Phase != v1.PodRunning && newPod.Status.Phase == v1.PodRunning {
				pl.serialGates[compType].Release(newPod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := podFromTombstone(obj)
			if pod == nil || !needSchedule(pod.Name) || !isPodScheduled(pod) {
				return
			}

			compType := componentType(pod)

			pl.serialGates[compType].Release(pod)
		},
	})
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	informerFactory := handle.SharedInformerFactory()
	cmLister := informerFactory.Core().V1().ConfigMaps().Lister()
	podLister := informerFactory.Core().V1().Pods().Lister()
	nodeLister := informerFactory.Core().V1().Nodes().Lister()

	trackerGateFactory := informers.NewSharedInformerFactory(handle.ClientSet(), 0)
	serialGates := make(map[string]*SerialGate, len(componentTypes))
	zoneTrackers := make(map[string]*ZoneCapacityTracker, len(componentTypes))
	for _, compType := range componentTypes {
		serialGates[compType] = NewSerialGate()
		zoneTrackers[compType] = NewZoneCapacityTracker()
	}

	nodeZonePlugin := &NodeZone{
		handle,
		cmLister,
		podLister,
		nodeLister,
		trackerGateFactory,
		zoneTrackers,
		serialGates,
	}

	nodeZonePlugin.registerNodeInformer(trackerGateFactory)
	nodeZonePlugin.registerPodInformer(trackerGateFactory)

	ctx := context.Background()
	trackerGateFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(
		ctx.Done(),
		trackerGateFactory.Core().V1().Nodes().Informer().HasSynced,
		trackerGateFactory.Core().V1().Pods().Informer().HasSynced,
	) {
		return nil, fmt.Errorf("node-zone: timed out waiting for cache sync")
	}

	return nodeZonePlugin, nil
}

func (pl *NodeZone) zoneForNodeName(nodeName string) string {
	nodeInfo, err := pl.handler.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return ""
	}
	return zoneForNode(nodeInfo.Node())
}
