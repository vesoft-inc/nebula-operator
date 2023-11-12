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

package component

import (
	"context"
	"fmt"
	"time"

	nebulago "github.com/vesoft-inc/nebula-go/v3/nebula"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	podutils "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	PVCProtectionFinalizer = "kubernetes.io/pvc-protection"
	RestartTolerancePeriod = time.Minute * 1
)

var (
	taints = []*corev1.Taint{
		{
			Key:    corev1.TaintNodeUnreachable,
			Effect: corev1.TaintEffectNoExecute,
		},
		{
			Key:    corev1.TaintNodeNotReady,
			Effect: corev1.TaintEffectNoExecute,
		},
	}
)

type storagedFailover struct {
	client.Client
	clientSet kube.ClientSet
}

func NewStoragedFailover(c client.Client, clientSet kube.ClientSet) FailoverManager {
	return &storagedFailover{Client: c, clientSet: clientSet}
}

func (s *storagedFailover) Failover(nc *v1alpha1.NebulaCluster) error {
	if err := s.tryRestartPod(nc); err != nil {
		return err
	}
	readyPods, err := s.toleratePods(nc)
	if err != nil {
		return err
	}
	if len(readyPods) == len(nc.Status.Storaged.FailureHosts) {
		return nil
	}
	if err := s.deleteFailureHost(nc); err != nil {
		return err
	}
	if err := s.deleteFailurePodAndPVC(nc); err != nil {
		return err
	}
	if err := s.checkPendingPod(nc); err != nil {
		return err
	}
	if !pointer.BoolDeref(nc.Status.Storaged.BalancedAfterFailover, false) {
		if err := s.balanceData(nc); err != nil {
			return err
		}
	}
	return nil
}

func (s *storagedFailover) Recovery(nc *v1alpha1.NebulaCluster) error {
	nc.Status.Storaged.FailureHosts = nil
	klog.Infof("clearing storaged cluster [%s/%s] failure hosts", nc.GetNamespace(), nc.GetName())
	return nil
}

func (s *storagedFailover) tryRestartPod(nc *v1alpha1.NebulaCluster) error {
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		if fh.PodRestarted {
			continue
		}
		if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
			return err
		}
		fh.PodRestarted = true
		nc.Status.Storaged.FailureHosts[podName] = fh
		return utilerrors.ReconcileErrorf("try to restart failure storaged pod [%s/%s] for recovery", nc.Namespace, podName)
	}
	return nil
}

func (s *storagedFailover) toleratePods(nc *v1alpha1.NebulaCluster) ([]string, error) {
	readyPods := make([]string, 0)
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		if fh.HostDeleted {
			continue
		}
		pod, err := s.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil {
			return nil, err
		}
		if pod != nil && isPodTerminating(pod) {
			node, err := s.clientSet.Node().GetNode(pod.Spec.NodeName)
			if err != nil {
				klog.Errorf("get node %s failed: %v", pod.Spec.NodeName, err)
				return nil, err
			}
			if isNodeDown(node) {
				fh.NodeDown = true
				nc.Status.Storaged.FailureHosts[podName] = fh
				continue
			}
			return nil, utilerrors.ReconcileErrorf("failure storaged pod [%s/%s] is deleting", nc.Namespace, podName)
		}
		if isPodHealthy(pod) {
			readyPods = append(readyPods, podName)
			continue
		}
		tolerance := pod.CreationTimestamp.Add(RestartTolerancePeriod)
		if time.Now().Before(tolerance) {
			return nil, utilerrors.ReconcileErrorf("waiting failure storaged pod [%s/%s] ready in tolerance period", nc.Namespace, podName)
		}
	}
	return readyPods, nil
}

func (s *storagedFailover) deleteFailureHost(nc *v1alpha1.NebulaCluster) error {
	ns := nc.GetNamespace()
	componentName := nc.StoragedComponent().GetName()
	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
	if err != nil {
		return err
	}
	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints, options...)
	if err != nil {
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			klog.Errorf("meta client disconnect failed: %v", err)
		}
	}()

	hosts := make([]*nebulago.HostAddr, 0)
	for _, fh := range nc.Status.Storaged.FailureHosts {
		if fh.HostDeleted {
			continue
		}
		count, err := metaClient.GetLeaderCount(fh.Host)
		if err != nil {
			klog.Errorf("storaged host %s get leader count failed: %v", fh.Host, err)
			return err
		}
		if count > 0 {
			return utilerrors.ReconcileErrorf("waiting for storaged host %s peers leader election done", fh.Host)
		}
		hosts = append(hosts, &nebulago.HostAddr{
			Host: fh.Host,
			Port: nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift),
		})
	}
	if len(hosts) == 0 {
		return nil
	}

	spaces, err := metaClient.ListSpaces()
	if err != nil {
		return err
	}

	if len(spaces) > 0 {
		if nc.Status.Storaged.RemovedSpaces == nil {
			nc.Status.Storaged.RemovedSpaces = make([]int32, 0, len(spaces))
		}
		nc.Status.Storaged.BalancedAfterFailover = pointer.Bool(false)
	}
	for _, space := range spaces {
		if contains(nc.Status.Storaged.RemovedSpaces, *space.Id.SpaceID) {
			continue
		}
		if err := removeHost(s.clientSet, metaClient, nc, *space.Id.SpaceID, hosts); err != nil {
			klog.Errorf("storaged cluster [%s/%s] remove failure hosts %v failed: %v", ns, componentName, hosts, err)
			return err
		}
		klog.Infof("storaged cluster [%s/%s] remove failure hosts %v in the space %s successfully", ns, componentName, hosts, space.Name)
	}

	for podName, fh := range nc.Status.Storaged.FailureHosts {
		if fh.HostDeleted {
			continue
		}
		fh.HostDeleted = true
		nc.Status.Storaged.FailureHosts[podName] = fh
	}

	nc.Status.Storaged.RemovedSpaces = nil
	nc.Status.Storaged.LastBalanceJob = nil
	return utilerrors.ReconcileErrorf("try to remove storaged cluster [%s/%s] failure host for recovery", ns, componentName)
}

func (s *storagedFailover) deleteFailurePodAndPVC(nc *v1alpha1.NebulaCluster) error {
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		if fh.PodRebuilt {
			continue
		}
		pod, pvcs, err := s.getPodAndPvcs(nc, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if pod == nil {
			klog.Infof("failure storaged pod [%s/%s] not found, skip", nc.Namespace, podName)
			continue
		}
		if !isPodTerminating(pod) {
			podScheduled := isPodConditionScheduledTrue(pod.Status.Conditions)
			klog.Infof("scheduled condition of pod [%s/%s] is %v", nc.Namespace, podName, podScheduled)
			if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
		} else {
			klog.Infof("pod [%s/%s] has DeletionTimestamp set to %s", nc.Namespace, podName, pod.DeletionTimestamp.String())
		}

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
		fh.PodRebuilt = true
		fh.DeletionTime = metav1.Time{Time: time.Now()}
		nc.Status.Storaged.FailureHosts[podName] = fh
		return utilerrors.ReconcileErrorf("try to delete failure storaged pod [%s/%s] for rebuilding", nc.Namespace, podName)
	}
	return nil
}

func (s *storagedFailover) checkPendingPod(nc *v1alpha1.NebulaCluster) error {
	for podName, fh := range nc.Status.Storaged.FailureHosts {
		pod, pvcs, err := s.getPodAndPvcs(nc, podName)
		if err != nil {
			return err
		}
		if pod == nil {
			return fmt.Errorf("rebuilt storaged pod [%s/%s] not found", nc.Namespace, podName)
		}
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
		if isPodConditionScheduledTrue(pod.Status.Conditions) && isPodPending(pod) {
			klog.Infof("storagd pod [%s/%s] conditions %v", nc.Namespace, podName, pod.Status.Conditions)
			if err := s.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
			return utilerrors.ReconcileErrorf("waiting for pending storaged pod [%s/%s] deleted", nc.Namespace, podName)
		}
	}
	return nil
}

func (s *storagedFailover) getPodAndPvcs(nc *v1alpha1.NebulaCluster, podName string) (*corev1.Pod, []corev1.PersistentVolumeClaim, error) {
	pod, err := s.clientSet.Pod().GetPod(nc.Namespace, podName)
	if err != nil {
		return nil, nil, err
	}
	pvcs, err := getPodPvcs(s.clientSet, nc, podName)
	if err != nil {
		return pod, nil, err
	}
	return pod, pvcs, nil
}

func (s *storagedFailover) balanceData(nc *v1alpha1.NebulaCluster) error {
	for podName := range nc.Status.Storaged.FailureHosts {
		pod, err := s.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil {
			return err
		}
		if !podutils.IsPodReady(pod) {
			return utilerrors.ReconcileErrorf("rebuilt storaged pod [%s/%s] is not ready", nc.Namespace, podName)
		}
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
		err := metaClient.Disconnect()
		if err != nil {
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
		if err := balanceSpace(s.clientSet, metaClient, nc, *space.Id.SpaceID); err != nil {
			return err
		}
	}

	nc.Status.Storaged.BalancedSpaces = nil
	nc.Status.Storaged.LastBalanceJob = nil
	nc.Status.Storaged.BalancedAfterFailover = pointer.Bool(true)
	return s.clientSet.NebulaCluster().UpdateNebulaCluster(nc)
}

func getPodPvcs(clientSet kube.ClientSet, nc *v1alpha1.NebulaCluster, podName string) ([]corev1.PersistentVolumeClaim, error) {
	cl := label.New().Cluster(nc.GetClusterName()).Storaged()
	cl[annotation.AnnPodNameKey] = podName
	pvcSelector, err := cl.Selector()
	if err != nil {
		return nil, err
	}
	pvcs, err := clientSet.PVC().ListPVCs(nc.Namespace, pvcSelector)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	return pvcs, nil
}

func isNodeDown(node *corev1.Node) bool {
	for i := range taints {
		if taintExists(node.Spec.Taints, taints[i]) {
			klog.Infof("node %s found taint %s", node.Name, taints[i].Key)
			return true
		}
	}
	if condition.IsNodeReadyFalseOrUnknown(&node.Status) {
		klog.Infof("node %s is not ready", node.Name)
		conditions := condition.GetNodeTrueConditions(&node.Status)
		for i := range conditions {
			klog.Infof("node %s condition type %s is true", conditions[i].Type)
		}
		return true
	}
	return false
}

func taintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}
