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

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	apivalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/webhook/util/validation"
)

// validateNebulaClusterGraphd validates a NebulaCluster for Graphd.
func validateNebulaClusterGraphd(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	replicas := *nc.Spec.Graphd.Replicas
	bHaMode := annotation.IsInHaMode(nc.Annotations)

	allErrs = append(allErrs, validation.ValidateMinReplicasGraphd(
		field.NewPath("spec").Child("graphd").Child("replicas"),
		int(replicas),
		bHaMode,
	)...)

	return allErrs
}

// validateNebulaClusterMetad validates a NebulaCluster for Metad.
func validateNebulaClusterMetad(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	replicas := *nc.Spec.Metad.Replicas
	bHaMode := annotation.IsInHaMode(nc.Annotations)

	allErrs = append(allErrs, validation.ValidateMinReplicasMetad(
		field.NewPath("spec").Child("metad").Child("replicas"),
		int(replicas),
		bHaMode,
	)...)

	return allErrs
}

// validateNebulaClusterStoraged validates a NebulaCluster for Storaged.
func validateNebulaClusterStoraged(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	replicas := *nc.Spec.Storaged.Replicas
	bHaMode := annotation.IsInHaMode(nc.Annotations)

	allErrs = append(allErrs, validation.ValidateMinReplicasStoraged(
		field.NewPath("spec").Child("storaged").Child("replicas"),
		int(replicas),
		bHaMode,
	)...)

	return allErrs
}

// validateNebulaClusterGraphd validates a NebulaCluster for Graphd on create.
func validateNebulaClusterCreateGraphd(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateNebulaClusterGraphd(nc)...)

	return allErrs
}

// validateNebulaClusterMetad validates a NebulaCluster for Metad on create.
func validateNebulaClusterCreateMetad(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateNebulaClusterMetad(nc)...)

	return allErrs
}

// validateNebulaClusterStoraged validates a NebulaCluster for Storaged on create.
func validateNebulaClusterCreateStoraged(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	allErrs = append(allErrs, validateNebulaClusterStoraged(nc)...)

	return allErrs
}

// ValidateNebulaCluster validates a NebulaCluster on create.
func validateNebulaClusterCreate(nc *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	name := nc.Name
	namespace := nc.Namespace

	klog.Infof("receive admission to %s NebulaCluster[%s/%s]", "create", namespace, name)

	allErrs = append(allErrs, validateNebulaClusterCreateGraphd(nc)...)
	allErrs = append(allErrs, validateNebulaClusterCreateMetad(nc)...)
	allErrs = append(allErrs, validateNebulaClusterCreateStoraged(nc)...)

	return allErrs
}

// validateNebulaClusterGraphd validates a NebulaCluster for Graphd on update.
func validateNebulaClusterUpdateGraphd(nc, oldNC *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	_ = oldNC // unused
	allErrs = append(allErrs, validateNebulaClusterGraphd(nc)...)

	return allErrs
}

// validateNebulaClusterMetad validates a NebulaCluster for Metad on Update.
func validateNebulaClusterUpdateMetad(nc, oldNC *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(
		nc.Spec.Metad.Replicas,
		oldNC.Spec.Metad.Replicas,
		field.NewPath("spec").Child("metad").Child("replicas"),
	)...)
	allErrs = append(allErrs, validateNebulaClusterMetad(nc)...)

	return allErrs
}

// validateNebulaClusterStoraged validates a NebulaCluster for Storaged on Update.
func validateNebulaClusterUpdateStoraged(nc, oldNC *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	klog.Infof(
		"Phase = %s, Replicas %d -> %d",
		nc.Status.Storaged.Phase,
		*oldNC.Spec.Storaged.Replicas,
		*nc.Spec.Storaged.Replicas,
	)
	if nc.Status.Storaged.Phase != v1alpha1.RunningPhase {
		if *nc.Spec.Storaged.Replicas != *oldNC.Spec.Storaged.Replicas {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "storaged", "replicas"),
				nc.Spec.Storaged.Replicas,
				fmt.Sprintf("field is immutable while not in %s phase", v1alpha1.RunningPhase),
			))
		}
	}

	allErrs = append(allErrs, validateNebulaClusterStoraged(nc)...)

	return allErrs
}

// ValidateNebulaCluster validates a NebulaCluster on Update.
func validateNebulaClusterUpdate(nc, oldNC *v1alpha1.NebulaCluster) (allErrs field.ErrorList) {
	name := nc.Name
	namespace := nc.Namespace

	klog.Infof("receive admission to %s NebulaCluster[%s/%s]", "update", namespace, name)

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(
		&nc.ObjectMeta,
		&oldNC.ObjectMeta,
		field.NewPath("metadata"),
	)...)

	allErrs = append(allErrs, apivalidation.ValidateImmutableAnnotation(
		nc.Annotations[annotation.AnnHaModeKey],
		oldNC.Annotations[annotation.AnnHaModeKey],
		annotation.AnnHaModeKey,
		field.NewPath("metadata"),
	)...)

	allErrs = append(allErrs, validateNebulaClusterUpdateGraphd(nc, oldNC)...)
	allErrs = append(allErrs, validateNebulaClusterUpdateMetad(nc, oldNC)...)
	allErrs = append(allErrs, validateNebulaClusterUpdateStoraged(nc, oldNC)...)

	return allErrs
}
