package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func (na *NebulaAutoscaler) GetPollingPeriod() *metav1.Duration {
	if na == nil {
		return nil
	}
	return na.Spec.PollingPeriod
}
