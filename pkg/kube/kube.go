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

package kube

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type FinalizerOpType string

const (
	AddFinalizerOpType    FinalizerOpType = "Add"
	RemoveFinalizerOpType FinalizerOpType = "Remove"
)

func UpdateFinalizer(ctx context.Context, c client.Client, object client.Object, op FinalizerOpType, finalizer string) error {
	switch op {
	case AddFinalizerOpType, RemoveFinalizerOpType:
	default:
		return fmt.Errorf("unsupported op type %s", op)
	}

	key := client.ObjectKeyFromObject(object)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fetchedObject := object.DeepCopyObject().(client.Object)
		if err := c.Get(ctx, key, fetchedObject); err != nil {
			return err
		}
		finalizers := fetchedObject.GetFinalizers()
		switch op {
		case AddFinalizerOpType:
			if controllerutil.ContainsFinalizer(fetchedObject, finalizer) {
				return nil
			}
			finalizers = append(finalizers, finalizer)
		case RemoveFinalizerOpType:
			finalizerSet := sets.NewString(finalizers...)
			if !finalizerSet.Has(finalizer) {
				return nil
			}
			finalizers = finalizerSet.Delete(finalizer).List()
		}
		fetchedObject.SetFinalizers(finalizers)
		return c.Update(ctx, fetchedObject)
	})
}

func ValidVersion(ver *version.Info) (bool, error) {
	major, err := strconv.Atoi(ver.Major)
	if err != nil {
		return false, err
	}
	leadingDigits := regexp.MustCompile(`^(\d+)`)
	minor, err := strconv.Atoi(leadingDigits.FindString(ver.Minor))
	if err != nil {
		return false, err
	}
	return major >= 1 && minor >= 18, nil
}
