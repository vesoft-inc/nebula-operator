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

package nebulaautoscaler

import (
	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/webhook/util/validation"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
)

// ValidateNebulaAutoscalerCreate validates a NebulaAutoscaler on create.
func validateNebulaAutoscalerCreate(na *v1alpha1.NebulaAutoscaler) (allErrs field.ErrorList) {
	name := na.Name
	namespace := na.Namespace

	klog.Infof("receive admission with resource [%s/%s], GVK %s, operation %s", namespace, name,
		na.GroupVersionKind().String(), admissionv1.Create)

	allErrs = append(allErrs, validateNebulaAutoscalarReplica(na)...)

	return allErrs
}

// ValidateNebulaCluster validates a NebulaAutoscaler on Update.
func validateNebulaAutoscalerUpdate(na, oldNA *v1alpha1.NebulaAutoscaler) (allErrs field.ErrorList) {
	name := na.Name
	namespace := na.Namespace

	klog.Infof("receive admission with resource [%s/%s], GVK %s, operation %s", namespace, name,
		na.GroupVersionKind().String(), admissionv1.Update)

	allErrs = append(allErrs, validateNebulaAutoscalarReplica(na)...)

	return allErrs
}

// validateNebulaClusterGraphd validates the replicas in an NebulaAutoscaler
func validateNebulaAutoscalarReplica(na *v1alpha1.NebulaAutoscaler) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validation.ValidateMinMaxReplica(
		field.NewPath("spec").Child("graphPolicy").Child("minReplicas"),
		int(*na.Spec.GraphdPolicy.MinReplicas),
		int(na.Spec.GraphdPolicy.MaxReplicas),
	)...)

	return allErrs
}
