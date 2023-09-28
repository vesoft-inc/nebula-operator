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

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

type (
	ClusterCtxValue struct {
		Name string
	}
	clusterCtxKey struct{}

	ClusterOption  func(*ClusterOptions)
	ClusterOptions struct {
		Name           string
		NamePrefix     string
		KindConfigPath string
	}
)

func (o *ClusterOptions) WithOptions(opts ...ClusterOption) *ClusterOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithClusterNamePrefix(namePrefix string) ClusterOption {
	return func(o *ClusterOptions) {
		o.NamePrefix = namePrefix
	}
}

func WithClusterKindConfigPath(kindConfigPath string) ClusterOption {
	return func(o *ClusterOptions) {
		o.KindConfigPath = kindConfigPath
	}
}

func GetClusterCtxValue(ctx context.Context) *ClusterCtxValue {
	v := ctx.Value(clusterCtxKey{})
	data, _ := v.(*ClusterCtxValue)
	return data
}

func InstallCluster(opts ...ClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&ClusterOptions{
			NamePrefix: "e2e",
		}).WithOptions(opts...)

		// If already have kubeconfig, then no need to create kubernetes cluster.
		if cfg.KubeconfigFile() != "" {
			klog.V(4).InfoS("Skipping install cluster and use an existing cluster", "kubeconfig", cfg.KubeconfigFile())
			return ctx, nil
		}

		if o.Name == "" {
			o.Name = envconf.RandomName(o.NamePrefix, 16)
		}

		klog.V(4).InfoS("Install cluster", "name", o.Name)

		ctx, err := envfuncs.CreateClusterWithConfig(
			kind.NewProvider(),
			o.Name,
			o.KindConfigPath,
		)(ctx, cfg)
		if err != nil {
			klog.ErrorS(err, "Install cluster failed", "name", o.Name)
			return ctx, err
		}

		return context.WithValue(ctx, clusterCtxKey{}, &ClusterCtxValue{
			Name: o.Name,
		}), nil
	}
}

func UninstallCluster(opts ...ClusterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&ClusterOptions{}).WithOptions(opts...)

		if o.Name == "" {
			value := GetClusterCtxValue(ctx)
			if value != nil {
				o.Name = value.Name
			}
		}

		if o.Name == "" {
			return ctx, nil
		}

		klog.V(4).InfoS("Uninstall cluster", "name", o.Name)

		ctx, err := envfuncs.DestroyCluster(o.Name)(ctx, cfg)
		if err != nil {
			return ctx, err
		}
		return context.WithValue(ctx, clusterCtxKey{}, &ClusterCtxValue{
			Name: o.Name,
		}), nil
	}
}
