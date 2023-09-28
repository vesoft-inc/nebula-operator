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

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

type (
	OperatorCtxValue struct {
		Name      string
		Namespace string
	}
	operatorCtxKey struct{}

	OperatorOption func(*OperatorOptions)

	OperatorOptions struct {
		HelmOptions []HelmOption
	}
)

func (o *OperatorOptions) WithOptions(opts ...OperatorOption) *OperatorOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithOperatorHelmOptions(opts ...HelmOption) OperatorOption {
	return func(o *OperatorOptions) {
		o.HelmOptions = append(o.HelmOptions, opts...)
	}
}

func WithOperatorHelmRawOptions(opts ...helm.Option) OperatorOption {
	return WithOperatorHelmOptions(WithHelmOptions(opts...))
}

func GetOperatorCtxValue(ctx context.Context) *OperatorCtxValue {
	v := ctx.Value(operatorCtxKey{})
	data, _ := v.(*OperatorCtxValue)
	return data
}

func InstallOperator(opts ...OperatorOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&OperatorOptions{}).
			WithOptions(
				WithOperatorHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		var err error
		ctx, err = HelmInstall(o.HelmOptions...)(ctx, cfg)
		if err != nil {
			return ctx, err
		}

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		return context.WithValue(ctx, operatorCtxKey{}, &OperatorCtxValue{
			Name:      rawHelmOpts.Name,
			Namespace: rawHelmOpts.Namespace,
		}), nil
	}
}

func UninstallOperator(opts ...OperatorOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&OperatorOptions{}).
			WithOptions(
				WithOperatorHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		var err error
		ctx, err = HelmUninstall(o.HelmOptions...)(ctx, cfg)
		if err != nil {
			return ctx, err
		}

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		return context.WithValue(ctx, operatorCtxKey{}, &OperatorCtxValue{
			Name:      rawHelmOpts.Name,
			Namespace: rawHelmOpts.Namespace,
		}), nil
	}
}
