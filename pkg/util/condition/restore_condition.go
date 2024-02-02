/*
Copyright 2024 Vesoft Inc.

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

func UpdateNebulaRestoreCondition(status *v1alpha1.RestoreStatus, condition *v1alpha1.RestoreCondition) bool {
	if condition == nil {
		return false
	}

	condition.LastTransitionTime = metav1.Now()
	conditionIndex, oldCondition := getRestoreCondition(status, condition.Type)

	if oldCondition == nil {
		status.Conditions = append(status.Conditions, *condition)
		return true
	}

	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isUpdate := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	status.Conditions[conditionIndex] = *condition

	return !isUpdate
}

func IsRestoreInvalid(restore *v1alpha1.NebulaRestore) bool {
	_, condition := getRestoreCondition(&restore.Status, v1alpha1.RestoreInvalid)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsRestoreMetadComplete(restore *v1alpha1.NebulaRestore) bool {
	_, condition := getRestoreCondition(&restore.Status, v1alpha1.RestoreMetadComplete)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsRestoreStoragedComplete(restore *v1alpha1.NebulaRestore) bool {
	_, condition := getRestoreCondition(&restore.Status, v1alpha1.RestoreStoragedCompleted)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsRestoreComplete(restore *v1alpha1.NebulaRestore) bool {
	_, condition := getRestoreCondition(&restore.Status, v1alpha1.RestoreComplete)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsRestoreFailed(restore *v1alpha1.NebulaRestore) bool {
	_, condition := getRestoreCondition(&restore.Status, v1alpha1.RestoreFailed)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func getRestoreCondition(status *v1alpha1.RestoreStatus, conditionType v1alpha1.RestoreConditionType) (int, *v1alpha1.RestoreCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}
