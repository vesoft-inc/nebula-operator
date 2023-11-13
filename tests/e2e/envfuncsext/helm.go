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

type HelmOption func(*HelmOptions)

type HelmOptions struct {
	HelmOptions []helm.Option
}

func (o *HelmOptions) WithOptions(opts ...HelmOption) *HelmOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *HelmOptions) RawHelmOpts() *helm.Opts {
	option := &helm.Opts{}
	for _, op := range o.HelmOptions {
		op(option)
	}
	return option
}

func WithHelmOptions(opts ...helm.Option) HelmOption {
	return func(o *HelmOptions) {
		o.HelmOptions = append(o.HelmOptions, opts...)
	}
}

func HelmInstall(opts ...HelmOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&HelmOptions{
			HelmOptions: []helm.Option{
				helm.WithWait(),
			},
		}).WithOptions(opts...)

		helmMgr := helm.New(cfg.KubeconfigFile())

		if err := helmMgr.RunInstall(o.HelmOptions...); err != nil {
			return ctx, err
		}

		return ctx, nil
	}
}

func HelmUpgrade(opts ...HelmOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&HelmOptions{
			HelmOptions: []helm.Option{
				helm.WithWait(),
			},
		}).WithOptions(opts...)

		helmMgr := helm.New(cfg.KubeconfigFile())

		if err := helmMgr.RunUpgrade(o.HelmOptions...); err != nil {
			return ctx, err
		}

		return ctx, nil
	}
}

func HelmUninstall(opts ...HelmOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&HelmOptions{
			HelmOptions: []helm.Option{},
		}).WithOptions(opts...)

		helmMgr := helm.New(cfg.KubeconfigFile())
		if err := helmMgr.RunUninstall(o.HelmOptions...); err != nil {
			return ctx, err
		}

		return ctx, nil
	}
}
