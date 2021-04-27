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

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	fmtNotHaModeErrorDetail = "should be at least %d not in ha mode"
	fmtHaModeErrorDetail    = "should be at least %d in ha mode"
	oddNumberDetail         = "should be odd number"
)

// ValidateMinReplicas validates replicas min value
func ValidateMinReplicas(fldPath *field.Path, actualValue, minValue int, bHaMode bool) *field.Error {
	if actualValue < minValue {
		detail := fmt.Sprintf(fmtNotHaModeErrorDetail, minValue)
		if bHaMode {
			detail = fmt.Sprintf(fmtHaModeErrorDetail, minValue)
		}
		return field.Invalid(fldPath, actualValue, detail)
	}
	return nil
}

// ValidateOddNumber validates odd number
func ValidateOddNumber(fldPath *field.Path, value int) *field.Error {
	if value&1 == 0 {
		return field.Invalid(fldPath, value, oddNumberDetail)
	}
	return nil
}
