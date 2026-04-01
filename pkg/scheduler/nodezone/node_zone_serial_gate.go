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
	"container/heap"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type AcquireResult int

const (
	AcquiredParallel AcquireResult = iota
	AcquiredSerial
	Waiting
)

// SerialGate coordinates parallel vs. ordered-serial scheduling.
// In parallel mode all calls to AcquireOrEnqueue return AcquiredParallel.
// In serial mode pods acquire the gate in pod-number order.
type SerialGate struct {
	mu      sync.Mutex
	active  bool        // true = serial mode
	holder  *PendingPod // pod currently holding the gate
	waiting podHeap     // min-heap ordered by pod number
}

func NewSerialGate() *SerialGate {
	return &SerialGate{
		waiting: podHeap{
			index: make(map[types.UID]int),
		},
	}
}

// SetSerial switches the gate between parallel and serial mode.
// Called by the ZoneCapacityTracker when zone criticality changes.
func (g *SerialGate) SetSerial(enable bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active == enable {
		return
	}
	g.active = enable
	if !enable {
		// Back to parallel — immediately approve all waiters.
		g.drainWaiters()
	}
}

// drainWaiters approves every waiting pod. Caller must hold g.mu.
func (g *SerialGate) drainWaiters() {
	for len(g.waiting.pods) > 0 {
		next := heap.Pop(&g.waiting).(*PendingPod)
		go next.approve()
	}
	g.holder = nil
}

// AcquireOrEnqueue is atomic: it either grants the gate or enqueues the pod.
func (g *SerialGate) AcquireOrEnqueue(pod *PendingPod) AcquireResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.active {
		return AcquiredParallel
	}

	klog.V(5).Infof("Pod [%v/%v-%v]: Heap status: %v", pod.namespace, pod.parent, pod.number, g.waiting.PendingPodsToString())
	klog.V(5).Infof("Pod [%v/%v-%v]: Map status: %v", pod.namespace, pod.parent, pod.number, g.waiting.index)

	if g.holder != nil {
		klog.V(5).Infof("Pod [%v/%v-%v]: in AcquireOrEnqueue current g.holder: %v-%v", pod.namespace, pod.parent, pod.number, g.holder.parent, g.holder.number)
		klog.V(5).Infof("Pod [%v/%v-%v]: Pod uid: %v, g.holder uid: %v", pod.namespace, pod.parent, pod.number, pod.uid, g.holder.uid)
		if g.holder.uid == pod.uid {
			return AcquiredSerial
		}
	}

	g.enqueue(pod)

	// Schedule a delayed advance if this is the first waiter.
	// The delay allows other pods in the same scheduling wave to enqueue
	// before we commit to an ordering decision.
	if g.waiting.pods[0].uid == pod.uid && g.holder == nil {
		go func() {
			time.Sleep(500 * time.Millisecond)
			g.mu.Lock()
			defer g.mu.Unlock()
			if g.active {
				g.TryAdvanceLocked(pod)
			}
		}()
	}

	return Waiting
}

func (g *SerialGate) TryAdvanceLocked(pod *PendingPod) {
	if g.holder != nil {
		klog.V(5).Infof("Pod [%v/%v-%v]: in TryAdvanceLocked current g.holder: %v-%v", pod.namespace, pod.parent, pod.number, g.holder.parent, g.holder.number)
	}

	if g.holder != nil || len(g.waiting.pods) == 0 {
		return
	}
	g.tryApproveNext(pod.namespace, fmt.Sprintf("%v-%v", pod.parent, pod.number))
}

// Release records the pod as deployed and advances the queue.
// Safe to call in parallel or serial mode; it is a no-op when inactive.
func (g *SerialGate) Release(pod *v1.Pod) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.active || g.holder == nil || g.holder.uid != pod.UID {
		var gholder string
		if g.holder != nil {
			gholder = fmt.Sprintf("%v-%v UID: %v", g.holder.parent, g.holder.number, g.holder.uid)
		}

		klog.V(5).Infof("Pod [%v/%v]: Release returned early g.active: %v, g.holder: %v, pod.UID: %v", pod.Namespace, pod.Name, g.active, gholder, pod.UID)
		return
	}

	g.holder = nil
	if len(g.waiting.pods) == 0 {
		return
	}
	g.tryApproveNext(pod.Namespace, pod.Name)
}

func (g *SerialGate) tryApproveNext(podNamesace string, podName string) {
	if !g.waiting.pods[0].waitingOnZone {
		klog.Infof("Pod [%v/%v]: unable to approve the next waiting pod: %v-%v because it's still waiting for an anchor/sibling pod",
			podNamesace, podName, g.waiting.pods[0].parent, g.waiting.pods[0].number)
		return
	}

	next := heap.Pop(&g.waiting).(*PendingPod)
	g.holder = next
	go next.approve() // fires wp.Allow async, which unblocks the Wait in Permit
}

func (g *SerialGate) Enqueue(pod *PendingPod) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.enqueue(pod)
}

// enqueue adds the pod to the heap if not already present. O(log n).
func (g *SerialGate) enqueue(pod *PendingPod) {
	if i, exists := g.waiting.index[pod.uid]; exists {
		if g.waiting.pods[i].waitingOnZone != pod.waitingOnZone {
			g.waiting.pods[i] = pod
		}
	} else {
		heap.Push(&g.waiting, pod)
	}
	// index updated by podHeap.Push
}

func (g *SerialGate) RemoveFromHeap(uid types.UID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeFromHeap(uid)
}

// removeFromHeap removes a pod by UID. O(log n).
func (g *SerialGate) removeFromHeap(uid types.UID) {
	i, exists := g.waiting.index[uid]
	if !exists {
		return
	}
	heap.Remove(&g.waiting, i)
	// index entry deleted by podHeap.Pop (called internally by heap.Remove)
}

type PendingPod struct {
	uid           types.UID
	namespace     string
	parent        string
	number        int
	waitingOnZone bool // true = waiting only on gate; false = also on anchor/sibling
	approve       func()
}

type podHeap struct {
	pods  []*PendingPod
	index map[types.UID]int // reference to SerialGate.index
}

func (h podHeap) Len() int { return len(h.pods) }

func (h podHeap) Less(i, j int) bool {
	return h.pods[i].number < h.pods[j].number
}

func (h podHeap) Swap(i, j int) {
	h.pods[i], h.pods[j] = h.pods[j], h.pods[i]
	// Keep index in sync on every swap.
	h.index[h.pods[i].uid] = i
	h.index[h.pods[j].uid] = j
}

func (h *podHeap) Push(x interface{}) {
	pod := x.(*PendingPod)
	h.index[pod.uid] = len(h.pods)
	h.pods = append(h.pods, pod)
}

func (h *podHeap) Pop() interface{} {
	old := h.pods
	n := len(old)
	pod := old[n-1]
	old[n-1] = nil
	h.pods = old[:n-1]
	delete(h.index, pod.uid)
	return pod
}

func (h *podHeap) PendingPodsToString() string {
	pendingPodsStr := "{"
	for i, pod := range h.pods {
		if i > 0 {
			pendingPodsStr += ", "
		}
		pendingPodsStr += fmt.Sprintf("%v-%v:%v", pod.parent, pod.number, pod.uid)
	}
	pendingPodsStr += "}"
	return pendingPodsStr
}
