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
	stderrors "errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type (
	NebulaClusterCtxValue struct {
		Name      string
		Namespace string
	}
	nebulaClusterCtxKey struct{}

	NebulaClusterOption  func(*NebulaClusterOptions)
	NebulaClusterOptions struct {
		HelmOptions []HelmOption
		ReadyFuncs  []NebulaClusterReadyFunc
		WaitOptions []wait.Option
	}

	NebulaClusterReadyFunc func(context.Context, *envconf.Config, *appsv1alpha1.NebulaCluster) (bool, error)
)

func (o *NebulaClusterOptions) WithOptions(opts ...NebulaClusterOption) *NebulaClusterOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithNebulaClusterHelmOptions(opts ...HelmOption) NebulaClusterOption {
	return func(o *NebulaClusterOptions) {
		o.HelmOptions = append(o.HelmOptions, opts...)
	}
}

func WithNebulaClusterHelmRawOptions(opts ...helm.Option) NebulaClusterOption {
	return WithNebulaClusterHelmOptions(WithHelmOptions(opts...))
}

func WithNebulaClusterReadyFuncs(fns ...NebulaClusterReadyFunc) NebulaClusterOption {
	return func(o *NebulaClusterOptions) {
		o.ReadyFuncs = append(o.ReadyFuncs, fns...)
	}
}

func WithNebulaClusterWaitOptions(opts ...wait.Option) NebulaClusterOption {
	return func(o *NebulaClusterOptions) {
		o.WaitOptions = append(o.WaitOptions, opts...)
	}
}

func GetNebulaClusterCtxValue(ctx context.Context) *NebulaClusterCtxValue {
	v := ctx.Value(nebulaClusterCtxKey{})
	data, _ := v.(*NebulaClusterCtxValue)
	return data
}

func InstallNebulaCluster(opts ...NebulaClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaClusterOptions{}).
			WithOptions(
				WithNebulaClusterHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		var err error
		ctx, err = HelmInstall(o.HelmOptions...)(ctx, cfg)
		if err != nil {
			return ctx, err
		}

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		return context.WithValue(ctx, nebulaClusterCtxKey{}, &NebulaClusterCtxValue{
			Name:      rawHelmOpts.Name,
			Namespace: rawHelmOpts.Namespace,
		}), nil
	}
}

func UpgradeNebulaCluster(opts ...NebulaClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaClusterOptions{}).
			WithOptions(
				WithNebulaClusterHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		var err error
		ctx, err = HelmUpgrade(o.HelmOptions...)(ctx, cfg)
		if err != nil {
			return ctx, err
		}

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		return context.WithValue(ctx, nebulaClusterCtxKey{}, &NebulaClusterCtxValue{
			Name:      rawHelmOpts.Name,
			Namespace: rawHelmOpts.Namespace,
		}), nil
	}
}

func WaitNebulaClusterReady(opts ...NebulaClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaClusterOptions{}).
			WithOptions(
				WithNebulaClusterHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		nc := &appsv1alpha1.NebulaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rawHelmOpts.Name,
				Namespace: rawHelmOpts.Namespace,
			},
		}
		if nc.Name == "" && nc.Namespace == "" {
			value := GetNebulaClusterCtxValue(ctx)
			if value == nil {
				klog.Error("Unset NebulaCluster")
				return ctx, stderrors.New("unset NebulaCluster")
			}

			nc.Name = value.Name
			nc.Namespace = value.Namespace
		}

		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			if err = cfg.Client().Resources().Get(ctx, nc.GetName(), nc.GetNamespace(), nc); err != nil {
				klog.ErrorS(err, "Get NebulaCluster failed", "namespace", nc.Namespace, "name", nc.Name)
				return false, err
			}
			klog.V(4).InfoS("Waiting for NebulaCluster to be ready",
				"namespace", nc.Namespace, "name", nc.Name,
				"generation", nc.Generation,
			)

			fns := o.ReadyFuncs
			if len(fns) == 0 {
				fns = append(fns, DefaultNebulaClusterReadyFunc)
			}

			for _, fn := range fns {
				if ok, err := fn(ctx, cfg, nc); err != nil || !ok {
					return ok, err
				}
			}
			return true, nil
		}, o.WaitOptions...); err != nil {
			ncCpy := nc.DeepCopy()
			ncCpy.ManagedFields = nil
			klog.ErrorS(err, "Waiting for NebulaCluster to be ready failed", "nc", ncCpy)
			return ctx, err
		}
		klog.InfoS("Waiting for NebulaCluster to be ready successfully", "namespace", nc.Namespace, "name", nc.Name)
		return ctx, nil
	}
}

func UninstallNebulaCluster(opts ...NebulaClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaClusterOptions{}).
			WithOptions(
				WithNebulaClusterHelmOptions(WithHelmOptions(
				// default values
				)),
			).WithOptions(opts...)

		var err error
		ctx, err = HelmUninstall(o.HelmOptions...)(ctx, cfg)
		if err != nil {
			return ctx, err
		}

		rawHelmOpts := (&HelmOptions{}).WithOptions(o.HelmOptions...).RawHelmOpts()
		return context.WithValue(ctx, nebulaClusterCtxKey{}, &NebulaClusterCtxValue{
			Name:      rawHelmOpts.Name,
			Namespace: rawHelmOpts.Namespace,
		}), nil
	}
}
