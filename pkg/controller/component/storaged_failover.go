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

package component

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	nebula0 "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	// PodScheduleTimeout is the duration to wait for a pod to be scheduled
	PodScheduleTimeout = 1 * time.Minute
	// RestartTolerancePeriod is the duration to wait for a pod to be ready after restart
	RestartTolerancePeriod = time.Minute * 1

	// PVCProtectionFinalizer is the finalizer for PVC protection
	PVCProtectionFinalizer = "kubernetes.io/pvc-protection"
)

// StoragedFailover handles failover operations for storaged pods
type storagedFailover struct {
	client.Client
	clientSet kube.ClientSet
}

func NewStoragedFailover(c client.Client, clientSet kube.ClientSet) FailoverManager {
	return &storagedFailover{Client: c, clientSet: clientSet}
}

// Failover handles the failover process for storaged pods
func (s *storagedFailover) Failover(nc *v1alpha1.NebulaCluster) error {
	// Step 1: Try to restart the failed pods
	if err := s.tryRestartPod(nc); err != nil {
		return err
	}

	// Step 2: Check if restarted pods are ready within a tolerance period
	readyPods, err := s.checkPodsAfterRestart(nc)
	if err != nil {
		return err
	}
	if len(readyPods) > 0 {
		return utilerrors.ReconcileErrorf("storaged pods [%v] are ready after restart", readyPods)
	}

	// Step 3: Delete failed pods and associated PVCs
	if err := s.deleteFailedPodAndPVC(nc); err != nil {
		return err
	}

	// Step 4: Check and handle pending pods
	if err := s.handlePendingPods(nc); err != nil {
		return err
	}

	// Step 5: Balance leaders across storage nodes
	return s.balanceStorageLeader(nc)
}

// Recovery clears failure status for given hosts
func (s *storagedFailover) Recovery(nc *v1alpha1.NebulaCluster, hosts []string) error {
	for _, host := range hosts {
		delete(nc.Status.Storaged.FailureHosts, host)
		klog.Infof("clearing storaged cluster [%s/%s] failure host %s", nc.GetNamespace(), nc.GetName(), host)
	}
	return nil
}

// tryRestartPod attempts to restart failed pods
func (s *storagedFailover) tryRestartPod(nc *v1alpha1.NebulaCluster) error {
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		// Skip if pod already restarted
		if fh.PodRestarted {
			continue
		}

		// Get pod information
		pod, err := s.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Skip if pod not found
		if pod == nil {
			continue
		}

		// Deal with pod in pending state
		if isPodPending(pod) {
			if err := s.checkPodAssociatedNodeStatus(nc, pod, &fh); err != nil {
				return err
			}
			continue
		}

		// Check if the pod is associated with a down node
		if pod.Spec.NodeName != "" {
			node, err := s.clientSet.Node().GetNode(pod.Spec.NodeName)
			if err != nil {
				klog.Errorf("get node %s failed: %v", pod.Spec.NodeName, err)
				return err
			}

			if node != nil && isNodeDown(node) {
				fh.NodeDown = true
				nc.Status.Storaged.FailureHosts[pod.Name] = fh
				klog.Infof("pod %s associated with node %s is not ready", pod.Name, pod.Spec.NodeName)
			}
		}

		// Delete pod to trigger restart
		klog.Infof("deleting pod %s/%s to trigger restart", nc.Namespace, podName)
		if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
			return err
		}

		// Update status
		fh.PodRestarted = true
		nc.Status.Storaged.FailureHosts[podName] = fh
		return utilerrors.ReconcileErrorf("try to restart failure storaged pod [%s/%s] for recovery",
			nc.Namespace, podName)
	}
	return nil
}

// checkPodAssociatedNodeStatus checks the status of a pod's node and updates the failure host status accordingly
func (s *storagedFailover) checkPodAssociatedNodeStatus(nc *v1alpha1.NebulaCluster, pod *corev1.Pod, fh *v1alpha1.FailureHost) error {
	if pod.Spec.NodeName != "" {
		node, err := s.clientSet.Node().GetNode(pod.Spec.NodeName)
		if err != nil {
			klog.Errorf("get node %s failed: %v", pod.Spec.NodeName, err)
			return err
		}

		if node != nil && isNodeDown(node) {
			fh.NodeDown = true
			nc.Status.Storaged.FailureHosts[pod.Name] = *fh
			klog.Infof("pending pod %s assigned to node %s is not ready", pod.Name, pod.Spec.NodeName)
			return utilerrors.ReconcileErrorf("detected node down for pending pod %s/%s", nc.Namespace, pod.Name)
		}
		return nil
	}

	ordinal := getPodOrdinal(pod.Name)
	pvcName := fmt.Sprintf("%s-data-%s-%d", nc.StoragedComponent().ComponentType(), nc.StoragedComponent().GetName(), ordinal)

	pvc, err := s.clientSet.PVC().GetPVC(nc.Namespace, pvcName)
	if err != nil && !apierrors.IsNotFound(err) {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("PVC %s/%s not found for pod %s", nc.Namespace, pvcName, pod.Name)
			return nil
		}
		return err
	}

	if pvc.Spec.VolumeName == "" {
		klog.V(4).Infof("PVC %s/%s has no associated volume yet", nc.Namespace, pvcName)
		return nil
	}

	pv, err := s.clientSet.PV().GetPersistentVolume(pvc.Spec.VolumeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("PV %s not found for PVC %s/%s", pvc.Spec.VolumeName, nc.Namespace, pvcName)
			return nil
		}
		return err
	}

	nodeName := getNodeNameFromPV(pv)
	if nodeName == "" {
		return nil
	}

	node, err := s.clientSet.Node().GetNode(nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Node %s not found for pod %s", nodeName, pod.Name)
			return nil
		}
		return err
	}

	if node != nil && isNodeDown(node) {
		fh.NodeDown = true
		nc.Status.Storaged.FailureHosts[pod.Name] = *fh
		klog.Infof("pending pod %s associated with node %s through PV is not ready", pod.Name, nodeName)
		return utilerrors.ReconcileErrorf("detected node down for pending pod %s/%s", nc.Namespace, pod.Name)
	}

	return nil
}

// getNodeNameFromPV extracts the node name from the PersistentVolume's NodeAffinity
func getNodeNameFromPV(pv *corev1.PersistentVolume) string {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return ""
	}

	for _, selector := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range selector.MatchExpressions {
			if expr.Key == "kubernetes.io/hostname" && len(expr.Values) > 0 {
				return expr.Values[0]
			}
		}
	}

	return ""
}

// checkPodsAfterRestart checks the status of restarted pods within a tolerance period
func (s *storagedFailover) checkPodsAfterRestart(nc *v1alpha1.NebulaCluster) ([]string, error) {
	readyPods := make([]string, 0)
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		// Skip if pod already rebuilt
		if fh.PodRebuilt {
			continue
		}

		// Get pod information
		pod, err := s.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		// Wait if the pod is terminating
		if pod != nil && isPodTerminating(pod) {
			return nil, utilerrors.ReconcileErrorf("failure storaged pod [%s/%s] is deleting", nc.Namespace, podName)
		}

		// Check if the pod is healthy
		if isPodHealthy(pod) {
			readyPods = append(readyPods, podName)
			continue
		}

		// Check if within tolerance period
		tolerance := pod.CreationTimestamp.Add(RestartTolerancePeriod)
		if time.Now().Before(tolerance) {
			return nil, utilerrors.ReconcileErrorf("waiting failure storaged pod [%s/%s] ready in tolerance period", nc.Namespace, podName)
		}
	}
	return readyPods, nil
}

// deleteFailedPodAndPVC handles deletion of failed pods and their PVCs
func (s *storagedFailover) deleteFailedPodAndPVC(nc *v1alpha1.NebulaCluster) error {
	cl := label.New().Cluster(nc.GetClusterName()).Storaged()

	failureHosts := make([]string, 0)
	for _, fh := range nc.Status.Storaged.FailureHosts {
		if !fh.PodRebuilt {
			failureHosts = append(failureHosts, fh.Host)
		}
	}

	exist, err := s.hasMultipleFailuresInSamePart(nc, failureHosts)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	for podName, fh := range nc.Status.Storaged.FailureHosts {
		// Skip if pod already rebuilt
		if fh.PodRebuilt {
			continue
		}

		// Only proceed if the node is down
		if !fh.NodeDown {
			klog.Infof("failure storaged pod [%s/%s] is not associated with a down node, skip", nc.Namespace, podName)
			continue
		}

		pod, pvcs, err := getPodAndPvcs(s.clientSet, nc, cl, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Skip if pod not found
		if pod == nil {
			klog.Infof("failure storaged pod [%s/%s] not found, skip", nc.Namespace, podName)
			continue
		}

		// Delete pod if not already terminating
		if !isPodTerminating(pod) {
			podScheduled := isPodConditionScheduledTrue(pod.Status.Conditions)
			klog.Infof("scheduled condition of pod [%s/%s] is %v", nc.Namespace, podName, podScheduled)
			if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
		} else {
			klog.Infof("pod [%s/%s] has DeletionTimestamp set to %s", nc.Namespace, podName, pod.DeletionTimestamp.String())
		}

		// Delete associated PVCs
		for i := range pvcs {
			pvc := pvcs[i]
			if _, exist := fh.PVCSet[pvc.UID]; exist {
				if pvc.DeletionTimestamp == nil {
					if err := s.clientSet.PVC().DeletePVC(nc.Namespace, pvc.Name); err != nil {
						return err
					}
					klog.Infof("delete failure storaged pod PVC [%s/%s] successfully", nc.Namespace, pvc.Name)
				} else {
					klog.Infof("PVC [%s/%s] has DeletionTimestamp set to %s", nc.Namespace, pvc.Name, pvc.DeletionTimestamp.String())
				}
			}
		}

		// Update status
		fh.PodRebuilt = true
		fh.DeletionTime = metav1.Time{Time: time.Now()}
		nc.Status.Storaged.FailureHosts[podName] = fh

		return utilerrors.ReconcileErrorf("try to delete failure storaged pod [%s/%s] for rebuilding", nc.Namespace, podName)
	}
	return nil
}

// handlePendingPods handles pods stuck in pending state
func (s *storagedFailover) handlePendingPods(nc *v1alpha1.NebulaCluster) error {
	cl := label.New().Cluster(nc.GetClusterName()).Storaged()

	for podName, fh := range nc.Status.Storaged.FailureHosts {
		if !fh.PodRebuilt {
			klog.Infof("failure storaged pod [%s/%s] not rebuilt, skip", nc.Namespace, podName)
			continue
		}
		pod, pvcs, err := getPodAndPvcs(s.clientSet, nc, cl, podName)
		if err != nil {
			return err
		}
		if pod == nil {
			return fmt.Errorf("rebuilt storaged pod [%s/%s] not found", nc.Namespace, podName)
		}

		// Handle PVC finalizers
		for i := range pvcs {
			pvc := pvcs[i]
			if _, exist := fh.PVCSet[pvc.UID]; exist {
				if pvc.DeletionTimestamp != nil && len(pvc.GetFinalizers()) > 0 {
					if err := kube.UpdateFinalizer(context.TODO(), s.Client, pvc.DeepCopy(), kube.RemoveFinalizerOpType, PVCProtectionFinalizer); err != nil {
						return err
					}
					return utilerrors.ReconcileErrorf("waiting for PVC [%s/%s] finalizer updated", nc.Namespace, pvc.Name)
				}
			}
		}

		// Handle pods stuck in pending state
		if isPodConditionScheduledTrue(pod.Status.Conditions) &&
			isPodPending(pod) &&
			time.Now().After(pod.CreationTimestamp.Add(PodScheduleTimeout)) {

			klog.Infof("storagd pod [%s/%s] conditions %v", nc.Namespace, podName, pod.Status.Conditions)
			if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
			return utilerrors.ReconcileErrorf("pending storaged pod [%s/%s] deleted, reschedule", nc.Namespace, podName)
		}
	}
	return nil
}

// balanceStorageLeader balances leaders across storage nodes
func (s *storagedFailover) balanceStorageLeader(nc *v1alpha1.NebulaCluster) error {
	if len(nc.Status.Storaged.FailureHosts) == 0 {
		klog.Infof("no failure hosts in storaged cluster [%s/%s], skip leader balance", nc.GetNamespace(), nc.GetName())
		return nil
	}

	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
	if err != nil {
		return err
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints, options...)
	if err != nil {
		klog.Errorf("create meta client failed: %v", err)
		return err
	}

	defer func() {
		if err := metaClient.Disconnect(); err != nil {
			klog.Errorf("disconnect meta client failed: %v", err)
		}
	}()

	spaces, err := metaClient.ListSpaces()
	if err != nil {
		return err
	}

	if len(spaces) > 0 && nc.Status.Storaged.BalancedSpaces == nil {
		nc.Status.Storaged.BalancedSpaces = make([]int32, 0, len(spaces))
	}

	for _, space := range spaces {
		if contains(nc.Status.Storaged.BalancedSpaces, *space.Id.SpaceID) {
			continue
		}
		if err := metaClient.BalanceLeader(*space.Id.SpaceID); err != nil {
			return err
		}
		nc.Status.Storaged.BalancedSpaces = append(nc.Status.Storaged.BalancedSpaces, *space.Id.SpaceID)
	}

	// Reset balance status
	nc.Status.Storaged.BalancedSpaces = nil
	nc.Status.Storaged.LastBalanceJob = nil

	return nil
}

// check if there are more than 2 failure hosts in the same part
func (s *storagedFailover) hasMultipleFailuresInSamePart(nc *v1alpha1.NebulaCluster, failureHosts []string) (bool, error) {
	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
	if err != nil {
		return false, err
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints, options...)
	if err != nil {
		klog.Errorf("create meta client failed: %v", err)
		return false, err
	}

	defer func() {
		if err := metaClient.Disconnect(); err != nil {
			klog.Errorf("disconnect meta client failed: %v", err)
		}
	}()

	spaceItems, err := metaClient.GetSpaceParts()
	if err != nil {
		return false, err
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	var foundResult int32
	resultCh := make(chan struct {
		found bool
		err   error
	}, 1)

	failureHostsSet := sets.NewString(failureHosts...)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // limit concurrency to 10

	for spaceID, parts := range spaceItems {
		for _, part := range parts {
			wg.Add(1)
			sem <- struct{}{} // occupy a concurrency slot
			go func(spaceID int32, part *meta.PartItem) {
				defer wg.Done()
				defer func() { <-sem }() // release concurrency slot

				// check if foundResult is already set
				if atomic.LoadInt32(&foundResult) == 1 {
					return
				}

				peers := convertNebulaHostAddr(part.Peers)
				peerSet := sets.NewString(peers...)
				interSection := peerSet.Intersection(failureHostsSet)

				if interSection.Len() >= 2 {
					if atomic.CompareAndSwapInt32(&foundResult, 0, 1) {
						klog.Infof("space %d part %d has more than 2 failure hosts: %v",
							spaceID, part.PartID, interSection.List())

						cancel()
						resultCh <- struct {
							found bool
							err   error
						}{true, nil}
					}
				}
			}(spaceID, part)
		}
	}

	// wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		if result.err != nil {
			return false, result.err
		}
		if result.found {
			return true, nil
		}
	}

	return false, nil
}

func convertNebulaHostAddr(peers []*nebula0.HostAddr) []string {
	hosts := make([]string, 0, len(peers))
	for _, peer := range peers {
		hosts = append(hosts, peer.Host)
	}
	return hosts
}
