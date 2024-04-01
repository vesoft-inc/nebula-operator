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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

const (
	PVCProtectionFinalizer = "kubernetes.io/pvc-protection"
	RestartTolerancePeriod = time.Minute * 1
)

func getPodOrdinal(name string) int {
	ordinal := -1
	s := strings.Split(name, "-")
	o := s[len(s)-1]
	if i, err := strconv.ParseInt(o, 10, 32); err == nil {
		ordinal = int(i)
	}
	return ordinal
}

func getPodAndPvcs(clientSet kube.ClientSet, nc *v1alpha1.NebulaCluster, cl label.Label, podName string) (*corev1.Pod, []corev1.PersistentVolumeClaim, error) {
	pod, err := clientSet.Pod().GetPod(nc.Namespace, podName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}
	cl[annotation.AnnPodNameKey] = podName
	pvcSelector, err := cl.Selector()
	if err != nil {
		return nil, nil, err
	}
	pvcs, err := clientSet.PVC().ListPVCs(nc.Namespace, pvcSelector)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}
	return pod, pvcs, nil
}

func isNodeDown(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		klog.Infof("node %s found taint %s, effect %s", node.Name, taint.Key, taint.Effect)
	}
	if condition.IsNodeReadyFalseOrUnknown(&node.Status) {
		klog.Infof("node %s is not ready", node.Name)
		conditions := condition.GetNodeTrueConditions(&node.Status)
		for i := range conditions {
			klog.Infof("node %s condition type %s is true", node.Name, conditions[i].Type)
		}
		return true
	}
	return false
}

func getWorkloadReplicas(workload *v1alpha1.WorkloadStatus) int32 {
	var workloadReplicas int32
	if workload != nil {
		workloadReplicas = workload.Replicas
	}
	return workloadReplicas
}
