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

package validating

import (
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	appsvalidation "k8s.io/kubernetes/pkg/apis/apps/validation"
	apivalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/webhook/util/validation"
)

// validateStatefulSetGraphd validates a StatefulSet for Graphd.
func validateStatefulSetGraphd(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	replicas := *statefulSet.Spec.Replicas
	bHaMode := annotation.IsInHaMode(statefulSet.Annotations)

	allErrs = append(allErrs, validateUpdateStrategy(statefulSet.Spec.UpdateStrategy)...)
	allErrs = append(allErrs, validation.ValidateMinReplicasGraphd(field.NewPath("spec").Child("replicas"), int(replicas), bHaMode)...)

	return allErrs
}

// validateStatefulSetMetad validates a StatefulSet for Metad.
func validateStatefulSetMetad(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	replicas := *statefulSet.Spec.Replicas
	bHaMode := annotation.IsInHaMode(statefulSet.Annotations)

	allErrs = append(allErrs, validateUpdateStrategy(statefulSet.Spec.UpdateStrategy)...)
	allErrs = append(allErrs, validation.ValidateMinReplicasMetad(field.NewPath("spec").Child("replicas"), int(replicas), bHaMode)...)

	return allErrs
}

// validateStatefulSetStoraged validates a StatefulSet for Storaged.
func validateStatefulSetStoraged(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	replicas := *statefulSet.Spec.Replicas
	bHaMode := annotation.IsInHaMode(statefulSet.Annotations)

	allErrs = append(allErrs, validateUpdateStrategy(statefulSet.Spec.UpdateStrategy)...)
	allErrs = append(allErrs, validation.ValidateMinReplicasStoraged(field.NewPath("spec").Child("replicas"), int(replicas), bHaMode)...)

	return allErrs
}

// validateStatefulSetGraphd validates a StatefulSet for Graphd on create.
func validateStatefulSetCreateGraphd(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateStatefulSetGraphd(statefulSet)...)

	return allErrs
}

// validateStatefulSetMetad validates a StatefulSet for Metad on create.
func validateStatefulSetCreateMetad(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateStatefulSetMetad(statefulSet)...)

	return allErrs
}

// validateStatefulSetStoraged validates a StatefulSet for Storaged on create.
func validateStatefulSetCreateStoraged(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateStatefulSetStoraged(statefulSet)...)

	return allErrs
}

// ValidateStatefulSet validates a StatefulSet on create.
func validateStatefulSetCreate(statefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	if !isManaged(statefulSet) {
		return nil
	}

	name := statefulSet.Name
	namespace := statefulSet.Namespace
	l := label.Label(statefulSet.Labels)

	klog.Infof("receive admission with resource [%s/%s], GVK %s, operation %s", namespace, name,
		statefulSet.GroupVersionKind().String(), admissionv1.Create)

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(
		&statefulSet.ObjectMeta,
		true,
		appsvalidation.ValidateStatefulSetName,
		field.NewPath("metadata"),
	)...)

	if l.IsGraphd() {
		return append(allErrs, validateStatefulSetCreateGraphd(statefulSet)...)
	} else if l.IsMetad() {
		return append(allErrs, validateStatefulSetCreateMetad(statefulSet)...)
	} else if l.IsStoraged() {
		return append(allErrs, validateStatefulSetCreateStoraged(statefulSet)...)
	}

	return allErrs
}

// validateStatefulSetGraphd validates a StatefulSet for Graphd on update.
func validateStatefulSetUpdateGraphd(statefulSet, oldStatefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	_ = oldStatefulSet // unused
	allErrs = append(allErrs, validateStatefulSetGraphd(statefulSet)...)

	return allErrs
}

// validateStatefulSetMetad validates a StatefulSet for Metad on Update.
func validateStatefulSetUpdateMetad(statefulSet, oldStatefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(
		statefulSet.Spec.Replicas,
		oldStatefulSet.Spec.Replicas,
		field.NewPath("spec").Child("replicas"),
	)...)
	allErrs = append(allErrs, validateStatefulSetMetad(statefulSet)...)

	return allErrs
}

// validateStatefulSetStoraged validates a StatefulSet for Storaged on Update.
func validateStatefulSetUpdateStoraged(statefulSet, oldStatefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	_ = oldStatefulSet // unused
	allErrs = append(allErrs, validateStatefulSetStoraged(statefulSet)...)

	return allErrs
}

// ValidateStatefulSet validates a StatefulSet on Update.
func validateStatefulSetUpdate(statefulSet, oldStatefulSet *appsv1.StatefulSet) (allErrs field.ErrorList) {
	if !(isManaged(statefulSet) || isManaged(oldStatefulSet)) {
		return nil
	}

	name := statefulSet.Name
	namespace := statefulSet.Namespace
	l := label.Label(statefulSet.Labels)

	klog.Infof("receive admission with resource [%s/%s], GVK %s, operation %s", namespace, name,
		statefulSet.GroupVersionKind().String(), admissionv1.Create)

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(
		&statefulSet.ObjectMeta,
		&oldStatefulSet.ObjectMeta,
		field.NewPath("metadata"),
	)...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableAnnotation(
		statefulSet.Annotations[annotation.AnnHaModeKey],
		oldStatefulSet.Annotations[annotation.AnnHaModeKey],
		annotation.AnnHaModeKey,
		field.NewPath("metadata"),
	)...)

	restoreReplicas := statefulSet.Spec.Replicas
	statefulSet.Spec.Replicas = oldStatefulSet.Spec.Replicas

	restoreTemplate := statefulSet.Spec.Template
	statefulSet.Spec.Template = oldStatefulSet.Spec.Template

	restoreStrategy := statefulSet.Spec.UpdateStrategy
	statefulSet.Spec.UpdateStrategy = oldStatefulSet.Spec.UpdateStrategy

	if !apiequality.Semantic.DeepEqual(statefulSet.Spec, oldStatefulSet.Spec) {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec"),
			"updates to statefulset spec for fields other than 'replicas', 'template', and 'updateStrategy' are forbidden",
		))
	}

	statefulSet.Spec.Replicas = restoreReplicas
	statefulSet.Spec.Template = restoreTemplate
	statefulSet.Spec.UpdateStrategy = restoreStrategy

	if l.IsGraphd() {
		return append(allErrs, validateStatefulSetUpdateGraphd(statefulSet, oldStatefulSet)...)
	} else if l.IsMetad() {
		return append(allErrs, validateStatefulSetUpdateMetad(statefulSet, oldStatefulSet)...)
	} else if l.IsStoraged() {
		return append(allErrs, validateStatefulSetUpdateStoraged(statefulSet, oldStatefulSet)...)
	}

	return allErrs
}

func validateUpdateStrategy(updateStrategy appsv1.StatefulSetUpdateStrategy) (allErrs field.ErrorList) {
	if updateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "updateStrategy", "type"),
			updateStrategy.Type,
			fmt.Sprintf("can not use %s updateStrategy type", appsv1.OnDeleteStatefulSetStrategyType)))
	}
	return allErrs
}

// isManaged returns whether StatefulSet is a Managerd
func isManaged(statefulSet *appsv1.StatefulSet) bool {
	name := statefulSet.Name
	namespace := statefulSet.Namespace
	l := label.Label(statefulSet.Labels)

	if !l.IsManagedByNebulaOperator() {
		klog.Infof("resource [%s/%s] not managed by Nebula Operator, admit", namespace, name)
		return false
	}

	if !(l.IsGraphd() || l.IsMetad() || l.IsStoraged()) {
		klog.Infof("resource [%s/%s] not Nebula component, admit", namespace, name)
		return false
	}

	return true
}
