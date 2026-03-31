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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")
	componentTypes   = []string{
		v1alpha1.GraphdComponentType.String(),
		v1alpha1.StoragedComponentType.String(),
	}
)

// getParentNameAndOrdinal gets the name of pod's parent StatefulSet and pod's ordinal as extracted from its Name.
func getParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// getParentName gets the name of pod's parent StatefulSet. If pod has not parented, the empty string is returned.
func getParentName(pod *corev1.Pod) string {
	parent, _ := getParentNameAndOrdinal(pod)
	return parent
}

// getConfigMapName gets the configmap name for zone data.
func getConfigMapName(pod *corev1.Pod) string {
	return fmt.Sprintf("%s-%s", getParentName(pod), v1alpha1.ZoneSuffix)
}

func needSchedule(podName string) bool {
	for _, component := range componentTypes {
		if strings.Contains(podName, component) {
			return true
		}
	}

	return false
}

func getPodNameByOrdinal(parentName string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parentName, ordinal)
}

func getNamespacedName(obj metav1.Object) string {
	return fmt.Sprintf("%v/%v", obj.GetNamespace(), obj.GetName())
}

func isPodScheduled(pod *corev1.Pod) bool {
	return pod != nil && pod.DeletionTimestamp == nil && pod.Spec.NodeName != ""
}

func zoneForNode(node *corev1.Node) string {
	if node == nil {
		return ""
	}
	if z, ok := node.Labels[corev1.LabelTopologyZone]; ok {
		return z
	}
	// Pre-1.17 fallback.
	return node.Labels["failure-domain.beta.kubernetes.io/zone"]
}

func isFinished(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodSucceeded ||
		pod.Status.Phase == v1.PodFailed ||
		pod.DeletionTimestamp != nil
}

func podFromTombstone(obj interface{}) *v1.Pod {
	if pod, ok := obj.(*v1.Pod); ok {
		return pod
	}
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil
	}
	pod, ok := tombstone.Obj.(*v1.Pod)
	if !ok {
		return nil
	}
	return pod
}

func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func componentType(pod *corev1.Pod) string {
	parent, _ := getParentNameAndOrdinal(pod)
	for _, component := range componentTypes {
		if strings.Contains(parent, component) {
			return component
		}
	}

	return parent
}
