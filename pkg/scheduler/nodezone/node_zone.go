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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodeZone is a plugin that checks node zone.
type NodeZone struct {
	cmLister corelisters.ConfigMapLister
}

var _ framework.PreFilterPlugin = &NodeZone{}
var _ framework.FilterPlugin = &NodeZone{}
var _ framework.EnqueueExtensions = &NodeZone{}

const (
	ConfigMap framework.GVK = "ConfigMap"

	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "NodeZone"

	// preFilterStateKey is the key in CycleState to NodeZone pre-computed data.
	// Using the name of the plugin will likely help us avoid collisions with other plugins.
	preFilterStateKey = "PreFilter" + Name

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
// TODO: upgrade to v1.28.0+
func (pl *NodeZone) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
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

// Filter invoked at the filter extension point.
func (pl *NodeZone) Filter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	if len(pod.Spec.TopologySpreadConstraints) == 0 {
		return nil
	}
	if !needSchedule(pod.Name) {
		return nil
	}

	state, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	zone, ok := nodeInfo.Node().GetLabels()[corev1.LabelTopologyZone]
	if !ok {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNoLabelTopologyZone)
	}

	stateZone := state[pod.Name]
	if stateZone != "" && stateZone != zone {
		klog.Infof("pod [%s/%s] not fit node %s", pod.Namespace, pod.Name, nodeInfo.Node().Name)
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReasonNotMatch)
	}

	return nil
}

// EventsToRegister returns the possible events that may make a Pod
// failed by this plugin schedulable.
func (pl *NodeZone) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		{Resource: ConfigMap, ActionType: framework.Add},
		{Resource: framework.Node, ActionType: framework.All},
	}
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	informerFactory := handle.SharedInformerFactory()
	cmLister := informerFactory.Core().V1().ConfigMaps().Lister()
	return &NodeZone{
		cmLister,
	}, nil
}
