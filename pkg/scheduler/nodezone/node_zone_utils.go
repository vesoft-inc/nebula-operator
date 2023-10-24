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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

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
	return strings.Contains(podName, v1alpha1.GraphdComponentType.String()) ||
		strings.Contains(podName, v1alpha1.StoragedComponentType.String())
}

func getPodNameByOrdinal(parentName string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parentName, ordinal)
}

func getNamespacedName(obj metav1.Object) string {
	return fmt.Sprintf("%v/%v", obj.GetNamespace(), obj.GetName())
}
