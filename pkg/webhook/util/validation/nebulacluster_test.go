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
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateMinReplicas_Common(t *testing.T) {
	testCases := []struct {
		name        string
		fn          func(fldPath *field.Path, replicas int, bHaMode bool) field.ErrorList
		path        *field.Path
		replicas    int
		bHaMode     bool
		expectError field.ErrorList
	}{
		/* Graphd not ha mode */ {
			name:     "Graphd: not ha mode: lt",
			fn:       ValidateMinReplicasGraphd,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasGraphdNotInHaMode - 1,
			bHaMode:  false,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasGraphdNotInHaMode-1,
					fmt.Sprintf(fmtNotHaModeErrorDetail, minReplicasGraphdNotInHaMode),
				),
			},
		}, {
			name:        "Graphd: not ha mode: eq",
			fn:          ValidateMinReplicasGraphd,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasGraphdNotInHaMode,
			bHaMode:     false,
			expectError: nil,
		}, {
			name:        "Graphd: not ha mode: gt",
			fn:          ValidateMinReplicasGraphd,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasGraphdNotInHaMode + 1,
			bHaMode:     false,
			expectError: nil,
		}, /* Graphd ha mode */ {
			name:     "Graphd: ha mode: lt",
			fn:       ValidateMinReplicasGraphd,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasGraphdInHaMode - 1,
			bHaMode:  true,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasGraphdInHaMode-1,
					fmt.Sprintf(fmtHaModeErrorDetail, minReplicasGraphdInHaMode),
				),
			},
		}, {
			name:        "Graphd: ha mode: eq",
			fn:          ValidateMinReplicasGraphd,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasGraphdInHaMode,
			bHaMode:     true,
			expectError: nil,
		}, {
			name:        "Graphd: ha mode: gt",
			fn:          ValidateMinReplicasGraphd,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasGraphdInHaMode + 1,
			bHaMode:     true,
			expectError: nil,
		}, /* Metad not ha mode */ {
			name:     "Metad: not ha mode: lt",
			fn:       ValidateMinReplicasMetad,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasMetadNotInHaMode - 1,
			bHaMode:  false,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadNotInHaMode-1,
					fmt.Sprintf(fmtNotHaModeErrorDetail, minReplicasMetadNotInHaMode),
				),
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadNotInHaMode-1,
					oddNumberDetail,
				),
			},
		}, {
			name:        "Metad: not ha mode: eq",
			fn:          ValidateMinReplicasMetad,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasMetadNotInHaMode,
			bHaMode:     false,
			expectError: nil,
		}, {
			name:     "Metad: not ha mode: gt",
			fn:       ValidateMinReplicasMetad,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasMetadNotInHaMode + 1,
			bHaMode:  false,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadNotInHaMode+1,
					oddNumberDetail,
				),
			},
		}, {
			name:        "Metad: not ha mode: gt",
			fn:          ValidateMinReplicasMetad,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasMetadNotInHaMode + 2,
			bHaMode:     false,
			expectError: nil,
		}, /* Metad ha mode */ {
			name:     "Metad: ha mode: lt",
			fn:       ValidateMinReplicasMetad,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasMetadInHaMode - 1,
			bHaMode:  true,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadInHaMode-1,
					fmt.Sprintf(fmtHaModeErrorDetail, minReplicasMetadInHaMode),
				),
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadInHaMode-1,
					oddNumberDetail,
				),
			},
		}, {
			name:        "Metad: ha mode: eq",
			fn:          ValidateMinReplicasMetad,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasMetadInHaMode,
			bHaMode:     true,
			expectError: nil,
		}, {
			name:     "Metad: ha mode: gt",
			fn:       ValidateMinReplicasMetad,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasMetadInHaMode + 1,
			bHaMode:  true,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasMetadInHaMode+1,
					oddNumberDetail,
				),
			},
		}, {
			name:        "Metad: ha mode: gt2",
			fn:          ValidateMinReplicasMetad,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasMetadInHaMode + 2,
			bHaMode:     true,
			expectError: nil,
		}, /* Storaged not ha mode */ {
			name:     "Storaged: not ha mode: lt",
			fn:       ValidateMinReplicasStoraged,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasStoragedNotInHaMode - 1,
			bHaMode:  false,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasStoragedNotInHaMode-1,
					fmt.Sprintf(fmtNotHaModeErrorDetail, minReplicasStoragedNotInHaMode),
				),
			},
		}, {
			name:        "Storaged: not ha mode: eq",
			fn:          ValidateMinReplicasStoraged,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasStoragedNotInHaMode,
			bHaMode:     false,
			expectError: nil,
		}, {
			name:        "Storaged: not ha mode: gt",
			fn:          ValidateMinReplicasStoraged,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasStoragedNotInHaMode + 1,
			bHaMode:     false,
			expectError: nil,
		}, /* Storaged ha mode */ {
			name:     "Storaged: ha mode: lt",
			fn:       ValidateMinReplicasStoraged,
			path:     field.NewPath("spec", "replicas"),
			replicas: minReplicasStoragedInHaMode - 1,
			bHaMode:  true,
			expectError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "replicas"),
					minReplicasStoragedInHaMode-1,
					fmt.Sprintf(fmtHaModeErrorDetail, minReplicasStoragedInHaMode),
				),
			},
		}, {
			name:        "Storaged: ha mode: eq",
			fn:          ValidateMinReplicasStoraged,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasStoragedInHaMode,
			bHaMode:     true,
			expectError: nil,
		}, {
			name:        "Storaged: ha mode: gt",
			fn:          ValidateMinReplicasStoraged,
			path:        field.NewPath("spec", "replicas"),
			replicas:    minReplicasStoragedInHaMode + 1,
			bHaMode:     true,
			expectError: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.fn(tc.path, tc.replicas, tc.bHaMode)
			if !reflect.DeepEqual(tc.expectError, actualError) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectError, actualError)
			}
		})
	}
}
