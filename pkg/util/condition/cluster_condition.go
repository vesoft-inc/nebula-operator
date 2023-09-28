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

package condition

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

const (
	WorkloadReady = "Ready"
	// WorkloadNotUpToDate is added when one of workloads is not up-to-date.
	WorkloadNotUpToDate = "WorkloadNotUpToDate"
	// MetadUnhealthy is added when one of metad pods is unhealthy.
	MetadUnhealthy = "MetadUnhealthy"
	// StoragedUnhealthy is added when one of storaged pods is unhealthy.
	StoragedUnhealthy = "StoragedUnhealthy"
	// GraphdUnhealthy is added when one of graphd pods is unhealthy.
	GraphdUnhealthy = "GraphdUnhealthy"
)

func NewNebulaClusterCondition(
	condType v1alpha1.NebulaClusterConditionType,
	status corev1.ConditionStatus,
	reason, message string,
) *v1alpha1.NebulaClusterCondition {
	return &v1alpha1.NebulaClusterCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func IsNebulaClusterReady(nc *v1alpha1.NebulaCluster) bool {
	for _, condition := range nc.Status.Conditions {
		if condition.Type == v1alpha1.NebulaClusterReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func GetNebulaClusterCondition(
	status *v1alpha1.NebulaClusterStatus,
	condType v1alpha1.NebulaClusterConditionType,
) *v1alpha1.NebulaClusterCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func SetNebulaClusterCondition(status *v1alpha1.NebulaClusterStatus, condition *v1alpha1.NebulaClusterCondition) {
	currentCond := GetNebulaClusterCondition(status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	status.Conditions = append(filterOutCondition(status.Conditions, condition.Type), *condition)
}

func filterOutCondition(
	conditions []v1alpha1.NebulaClusterCondition,
	condType v1alpha1.NebulaClusterConditionType,
) []v1alpha1.NebulaClusterCondition {
	var newConditions []v1alpha1.NebulaClusterCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
