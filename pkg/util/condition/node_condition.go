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

package condition

import (
	corev1 "k8s.io/api/core/v1"
)

func GetNodeTrueConditions(status *corev1.NodeStatus) []corev1.NodeCondition {
	if status == nil {
		return nil
	}
	return filterOutNodeConditionByStatus(status.Conditions, corev1.ConditionTrue)
}

func IsNodeReadyFalseOrUnknown(status *corev1.NodeStatus) bool {
	condition := getNodeReadyCondition(status)
	return condition != nil && (condition.Status == corev1.ConditionFalse || condition.Status == corev1.ConditionUnknown)
}

func getNodeReadyCondition(status *corev1.NodeStatus) *corev1.NodeCondition {
	if status == nil {
		return nil
	}
	condition := filterOutNodeConditionByType(status.Conditions, corev1.NodeReady)
	return condition
}

func filterOutNodeConditionByType(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func filterOutNodeConditionByStatus(conditions []corev1.NodeCondition, conditionStatus corev1.ConditionStatus) []corev1.NodeCondition {
	filtered := make([]corev1.NodeCondition, 0)
	for i := range conditions {
		if conditions[i].Status == conditionStatus {
			filtered = append(filtered, conditions[i])
		}
	}
	return filtered
}
