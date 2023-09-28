/*
Copyright 2023 Vesoft Inc.

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

package envfuncsext

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func IgnoreErrors(ctx context.Context, err error, isErrors ...func(error) bool) env.Func {
	return func(_ context.Context, _ *envconf.Config) (context.Context, error) {
		for _, isError := range isErrors {
			if isError(err) {
				return ctx, nil
			}
		}
		return ctx, err
	}
}

func IgnoreAlreadyExists(ctx context.Context, err error) env.Func {
	return IgnoreErrors(ctx, err, apierrors.IsAlreadyExists)
}

func IgnoreNotFound(ctx context.Context, err error) env.Func {
	return IgnoreErrors(ctx, err, apierrors.IsNotFound)
}
