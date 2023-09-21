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

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

const (
	minReplicasGraphdNotInHaMode   = 1
	minReplicasGraphdInHaMode      = 2
	minReplicasMetadNotInHaMode    = 1
	minReplicasMetadInHaMode       = 3
	minReplicasStoragedNotInHaMode = 1
	minReplicasStoragedInHaMode    = 3
)

// ValidateMinReplicasGraphd validates replicas min value for Graphd
func ValidateMinReplicasGraphd(fldPath *field.Path, replicas int, bHaMode bool) (allErrs field.ErrorList) {
	minReplicas := minReplicasGraphdNotInHaMode
	if bHaMode {
		minReplicas = minReplicasGraphdInHaMode
	}
	if fieldErr := ValidateMinReplicas(fldPath, replicas, minReplicas, bHaMode); fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}

	return allErrs
}

// ValidateMinReplicasMetad validates replicas min value for Metad
func ValidateMinReplicasMetad(fldPath *field.Path, replicas int, bHaMode bool) (allErrs field.ErrorList) {
	minReplicas := minReplicasMetadNotInHaMode
	if bHaMode {
		minReplicas = minReplicasMetadInHaMode
	}

	if fieldErr := ValidateMinReplicas(fldPath, replicas, minReplicas, bHaMode); fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}
	if replicas&1 == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, replicas, "should be odd number"))
	}

	return allErrs
}

// ValidateMinReplicasStoraged validates replicas min value for Storaged
func ValidateMinReplicasStoraged(fldPath *field.Path, replicas int, bHaMode bool) (allErrs field.ErrorList) {
	minReplicas := minReplicasStoragedNotInHaMode
	if bHaMode {
		minReplicas = minReplicasStoragedInHaMode
	}

	if fieldErr := ValidateMinReplicas(fldPath, replicas, minReplicas, bHaMode); fieldErr != nil {
		allErrs = append(allErrs, fieldErr)
	}

	return allErrs
}

func IsNebulaClusterHA(nc *v1alpha1.NebulaCluster) bool {
	if int(*nc.Spec.Graphd.Replicas) < minReplicasGraphdInHaMode {
		return false
	}
	if int(*nc.Spec.Metad.Replicas) < minReplicasMetadInHaMode {
		return false
	}
	if int(*nc.Spec.Storaged.Replicas) < minReplicasStoragedInHaMode {
		return false
	}
	return true
}
