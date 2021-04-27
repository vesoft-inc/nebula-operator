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

func TestValidateMinReplicas(t *testing.T) {
	testCases := []struct {
		name        string
		path        *field.Path
		actualValue int
		minValue    int
		bHaMode     bool
		expectError *field.Error
	}{
		{
			name:        "not ha mode: lt ",
			path:        field.NewPath("spec", "replicas"),
			actualValue: 2,
			minValue:    3,
			bHaMode:     false,
			expectError: field.Invalid(
				field.NewPath("spec", "replicas"),
				2,
				fmt.Sprintf(fmtNotHaModeErrorDetail, 3)),
		}, {
			name:        "ha mode: lt",
			path:        field.NewPath("spec", "replicas"),
			actualValue: 2,
			minValue:    3,
			bHaMode:     true,
			expectError: field.Invalid(
				field.NewPath("spec", "replicas"),
				2,
				fmt.Sprintf(fmtHaModeErrorDetail, 3)),
		}, {
			name:        "eq",
			path:        field.NewPath("spec", "replicas"),
			actualValue: 3,
			minValue:    3,
			bHaMode:     true,
			expectError: nil,
		}, {
			name:        "gt",
			path:        field.NewPath("spec", "replicas"),
			actualValue: 4,
			minValue:    3,
			bHaMode:     true,
			expectError: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := ValidateMinReplicas(tc.path, tc.actualValue, tc.minValue, tc.bHaMode)
			if !reflect.DeepEqual(tc.expectError, actualError) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectError, actualError)
			}
		})
	}
}

func TestValidateOddNumber(t *testing.T) {
	testCases := []struct {
		name        string
		path        *field.Path
		value       int
		expectError *field.Error
	}{
		{
			name:        "odd number",
			path:        field.NewPath("spec", "replicas"),
			value:       3,
			expectError: nil,
		}, {
			name:  "even number",
			path:  field.NewPath("spec", "replicas"),
			value: 4,
			expectError: field.Invalid(
				field.NewPath("spec", "replicas"),
				4,
				oddNumberDetail,
			),
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := ValidateOddNumber(tc.path, tc.value)
			if !reflect.DeepEqual(tc.expectError, actualError) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectError, actualError)
			}
		})
	}
}
