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

package errors

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/api/errors"
)

var _ error = &ReconcileError{}

type ReconcileError struct {
	Msg string
}

func (r *ReconcileError) Error() string {
	return r.Msg
}

func ReconcileErrorf(format string, a ...interface{}) error {
	return &ReconcileError{fmt.Sprintf(format, a...)}
}

func IsReconcileError(err error) bool {
	_, ok := err.(*ReconcileError)
	return ok
}

func IsStatusError(err error) bool {
	_, ok := err.(*errors.StatusError)
	return ok
}

func IsDNSError(err error) bool {
	_, ok := err.(*net.DNSError)
	return ok
}
