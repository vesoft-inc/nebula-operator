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
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// ZoneCapacityTracker maintains ground-truth node counts per zone and an
// optimistic decrement for each scheduled-but-not-yet-reflected pod.
type ZoneCapacityTracker struct {
	mu                sync.RWMutex
	nodes             map[string]*NodeInfo // nodeName -> NodeInfo
	podDecrement      map[string]int       // zone -> optimistic scheduled pods
	decrementedPods   map[types.UID]string // podUID -> zone (prevents double-counting)
	criticalThreshold int                  // threshold for when the switch to serial scheduling.
	parallelThreshold int                  // threshold for when we can go back to parallel scheduling.
}

func NewZoneCapacityTracker() *ZoneCapacityTracker {
	return &ZoneCapacityTracker{
		nodes:           make(map[string]*NodeInfo),
		podDecrement:    make(map[string]int),
		decrementedPods: make(map[types.UID]string),
	}
}

func (t *ZoneCapacityTracker) ZoneStatus(heading string, m *podMatcher, pod *v1.Pod) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for zone := range t.distinctZones() {
		klog.Infof("%v zone_%v count: %v", heading, zone, t.availableInZone(zone, m.Matches, pod))
	}
}

// countInZone counts nodes in zone that satisfy matchFn. Caller holds at least RLock.
func (t *ZoneCapacityTracker) countInZone(zone string, matchFn func(*NodeInfo) bool) int {
	n := 0
	for _, info := range t.nodes {
		if info.zone == zone && matchFn(info) {
			n++
		}
	}
	return n
}

// availableInZone returns eligible node count minus optimistic decrements.
func (t *ZoneCapacityTracker) availableInZone(zone string, matchFn func(*NodeInfo) bool, pod *v1.Pod) int {
	key := decrementKey(zone, pod)
	return t.countInZone(zone, matchFn) - t.podDecrement[key]
}

// distinctZones returns all zones present in the node set. Caller holds at least RLock.
func (t *ZoneCapacityTracker) distinctZones() map[string]struct{} {
	zones := make(map[string]struct{})
	for _, info := range t.nodes {
		zones[info.zone] = struct{}{}
	}
	return zones
}

// AnyCriticalForPod returns true if any zone is at or below criticalThreshold
// for this pod's constraints.
func (t *ZoneCapacityTracker) AnyCriticalForPod(m *podMatcher, pod *v1.Pod) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for zone := range t.distinctZones() {
		if t.availableInZone(zone, m.Matches, pod) <= criticalThreshold {
			return true
		}
	}
	return false
}

func (t *ZoneCapacityTracker) AddNode(node *v1.Node) {
	zone := zoneForNode(node)
	if zone == "" {
		return
	}
	t.mu.Lock()
	t.nodes[node.Name] = &NodeInfo{
		name:   node.Name,
		zone:   zone,
		labels: copyLabels(node.Labels),
	}
	t.mu.Unlock()
}

func (t *ZoneCapacityTracker) RemoveNode(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		node = tombstone.Obj.(*v1.Node)
	}
	t.mu.Lock()
	delete(t.nodes, node.Name)
	t.mu.Unlock()
}

// EnsureDecremented records an optimistic decrement when a pod is scheduled.
// Safe to call multiple times for the same pod.
func (t *ZoneCapacityTracker) EnsureDecremented(pod *v1.Pod, zone string) {
	key := decrementKey(zone, pod)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, already := t.decrementedPods[pod.UID]; already {
		return
	}
	t.decrementedPods[pod.UID] = key
	t.podDecrement[key]++
}

// EnsureIncremented reclaims the slot when a pod finishes or is deleted.
func (t *ZoneCapacityTracker) EnsureIncremented(pod *v1.Pod) {
	t.mu.Lock()
	key, ok := t.decrementedPods[pod.UID]
	if !ok {
		t.mu.Unlock()
		return
	}
	delete(t.decrementedPods, pod.UID)
	if t.podDecrement[key] > 0 {
		t.podDecrement[key]--
	}
	t.mu.Unlock()
}

// decrementKey returns a compound key of zone + StatefulSet parent name.
// This prevents pods from different StatefulSets (e.g. graphd vs storaged)
// from contaminating each other's zone decrements.
func decrementKey(zone string, pod *v1.Pod) string {
	parent, _ := getParentNameAndOrdinal(pod)
	if parent == "" {
		// Pod doesn't match the StatefulSet naming convention — use pod name
		// as the discriminator so it at least doesn't contaminate others.
		return zone + "/" + pod.Name
	}
	return zone + "/" + parent
}
