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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

type graphdFailover struct {
	client.Client
	clientSet kube.ClientSet
}

func NewGraphdFailover(c client.Client, clientSet kube.ClientSet) FailoverManager {
	return &graphdFailover{Client: c, clientSet: clientSet}
}

func (g *graphdFailover) Failover(nc *v1alpha1.NebulaCluster) error {
	if err := g.tryRestartPod(nc); err != nil {
		return err
	}
	readyPods, err := g.toleratePods(nc)
	if err != nil {
		return err
	}
	if len(readyPods) > 0 {
		return utilerrors.ReconcileErrorf("graphd pods [%v] are ready after restarted", readyPods)
	}
	if err := g.deleteFailurePodAndPVC(nc); err != nil {
		return err
	}
	if err := g.checkPendingPod(nc); err != nil {
		return err
	}
	return nil
}

func (g *graphdFailover) Recovery(nc *v1alpha1.NebulaCluster, hosts []string) error {
	for _, host := range hosts {
		delete(nc.Status.Graphd.FailureHosts, host)
		klog.Infof("clearing graphd cluster [%s/%s] failure host %s", nc.GetNamespace(), nc.GetName(), host)
	}
	return nil
}

func (g *graphdFailover) tryRestartPod(nc *v1alpha1.NebulaCluster) error {
	for podName, fh := range nc.Status.Graphd.FailureHosts {
		if fh.PodRestarted {
			continue
		}
		pod, err := g.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if pod == nil || isPodPending(pod) {
			continue
		}
		node, err := g.clientSet.Node().GetNode(pod.Spec.NodeName)
		if err != nil {
			klog.Errorf("get node %s failed: %v", pod.Spec.NodeName, err)
			return err
		}
		if isNodeDown(node) {
			fh.NodeDown = true
		}
		if err := g.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
			return err
		}
		fh.PodRestarted = true
		nc.Status.Graphd.FailureHosts[podName] = fh
		return utilerrors.ReconcileErrorf("try to restart failure graphd pod [%s/%s] for recovery", nc.Namespace, podName)
	}
	return nil
}

func (g *graphdFailover) toleratePods(nc *v1alpha1.NebulaCluster) ([]string, error) {
	readyPods := make([]string, 0)
	for podName, fh := range nc.Status.Graphd.FailureHosts {
		if fh.PodRebuilt {
			continue
		}
		pod, err := g.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if pod != nil && isPodTerminating(pod) {
			return nil, utilerrors.ReconcileErrorf("failure graphd pod [%s/%s] is deleting", nc.Namespace, podName)
		}
		if isPodHealthy(pod) {
			readyPods = append(readyPods, podName)
			continue
		}
		tolerance := pod.CreationTimestamp.Add(RestartTolerancePeriod)
		if time.Now().Before(tolerance) {
			return nil, utilerrors.ReconcileErrorf("waiting failure graphd pod [%s/%s] ready in tolerance period", nc.Namespace, podName)
		}
	}
	return readyPods, nil
}

func (g *graphdFailover) deleteFailurePodAndPVC(nc *v1alpha1.NebulaCluster) error {
	cl := label.New().Cluster(nc.GetClusterName()).Graphd()
	for podName, fh := range nc.Status.Graphd.FailureHosts {
		if fh.PodRebuilt {
			continue
		}
		pod, pvcs, err := getPodAndPvcs(g.clientSet, nc, cl, podName)
		if err != nil {
			return err
		}
		if pod == nil {
			return fmt.Errorf("failure graphd pod [%s/%s] not found, skip", nc.Namespace, podName)
		}
		if !isPodTerminating(pod) {
			podScheduled := isPodConditionScheduledTrue(pod.Status.Conditions)
			klog.Infof("scheduled condition of pod [%s/%s] is %v", nc.Namespace, podName, podScheduled)
			if err := g.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
		} else {
			klog.Infof("pod [%s/%s] has DeletionTimestamp set to %s", nc.Namespace, podName, pod.DeletionTimestamp.String())
		}

		for i := range pvcs {
			pvc := pvcs[i]
			if _, exist := fh.PVCSet[pvc.UID]; exist {
				if pvc.DeletionTimestamp == nil {
					if err := g.clientSet.PVC().DeletePVC(nc.Namespace, pvc.Name); err != nil {
						return err
					}
					klog.Infof("delete failure graphd pod PVC [%s/%s] successfully", nc.Namespace, pvc.Name)
				} else {
					klog.Infof("PVC [%s/%s] has DeletionTimestamp set to %s", nc.Namespace, pvc.Name, pvc.DeletionTimestamp.String())
				}
			}
		}
		fh.PodRebuilt = true
		fh.DeletionTime = metav1.Time{Time: time.Now()}
		nc.Status.Graphd.FailureHosts[podName] = fh
		return utilerrors.ReconcileErrorf("try to delete failure graphd pod [%s/%s] for rebuilding", nc.Namespace, podName)
	}
	return nil
}

func (g *graphdFailover) checkPendingPod(nc *v1alpha1.NebulaCluster) error {
	cl := label.New().Cluster(nc.GetClusterName()).Graphd()
	for podName, fh := range nc.Status.Graphd.FailureHosts {
		pod, pvcs, err := getPodAndPvcs(g.clientSet, nc, cl, podName)
		if err != nil {
			return err
		}
		if pod == nil {
			return fmt.Errorf("rebuilt graphd pod [%s/%s] not found, skip", nc.Namespace, podName)
		}
		for i := range pvcs {
			pvc := pvcs[i]
			if _, exist := fh.PVCSet[pvc.UID]; exist {
				if pvc.DeletionTimestamp != nil && len(pvc.GetFinalizers()) > 0 {
					if err := kube.UpdateFinalizer(context.TODO(), g.Client, pvc.DeepCopy(), kube.RemoveFinalizerOpType, PVCProtectionFinalizer); err != nil {
						return err
					}
					return utilerrors.ReconcileErrorf("waiting for PVC [%s/%s] finalizer updated", nc.Namespace, pvc.Name)
				}
			}
		}
		if isPodConditionScheduledTrue(pod.Status.Conditions) && isPodPending(pod) && time.Now().After(pod.CreationTimestamp.Add(time.Minute*1)) {
			klog.Infof("graphd pod [%s/%s] conditions %v", nc.Namespace, podName, pod.Status.Conditions)
			if err := g.clientSet.Pod().DeletePod(nc.Namespace, podName, true); err != nil {
				return err
			}
			return utilerrors.ReconcileErrorf("pending graphd pod [%s/%s] deleted, reschedule", nc.Namespace, podName)
		}
	}
	return nil
}
