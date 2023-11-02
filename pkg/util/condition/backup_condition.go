package condition

import (
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateNebulaBackupCondition(status *v1alpha1.BackupStatus, condition *v1alpha1.BackupCondition) bool {
	if condition == nil {
		return false
	}

	condition.LastTransitionTime = metav1.Now()
	conditionIndex, oldCondition := getBackupCondition(status, condition.Type)

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

func IsBackupInvalid(backup *v1alpha1.NebulaBackup) bool {
	_, condition := getBackupCondition(&backup.Status, v1alpha1.BackupInvalid)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsBackupComplete(backup *v1alpha1.NebulaBackup) bool {
	_, condition := getBackupCondition(&backup.Status, v1alpha1.BackupComplete)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsBackupFailed(backup *v1alpha1.NebulaBackup) bool {
	_, condition := getBackupCondition(&backup.Status, v1alpha1.BackupFailed)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func getBackupCondition(status *v1alpha1.BackupStatus, conditionType v1alpha1.BackupConditionType) (int, *v1alpha1.BackupCondition) {
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
