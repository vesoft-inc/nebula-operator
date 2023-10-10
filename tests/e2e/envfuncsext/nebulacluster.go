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

func DefaultNebulaClusterReadyFunc(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := nc.IsReady()
	if isReady {
		// TODO: Add more checks

		{ // Graphd status checks
			if !isComponentStatusExpected(
				&nc.Status.Graphd,
				&nc.Spec.Graphd.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.GraphdComponentType,
			) {
				isReady = false
			}
		}

		{ // Metad status checks
			if !isComponentStatusExpected(
				&nc.Status.Metad,
				&nc.Spec.Metad.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.MetadComponentType,
			) {
				isReady = false
			}
		}

		{ // Storaged status checks
			if !isComponentStatusExpected(
				&nc.Status.Storaged.ComponentStatus,
				&nc.Spec.Storaged.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.StoragedComponentType,
			) {
				isReady = false
			}

			if !nc.Status.Storaged.HostsAdded {
				klog.InfoS("Waiting for NebulaCluster to be ready but HostsAdded is false",
					"graphdReplicas", int(*nc.Spec.Graphd.Replicas),
					"namespace", nc.Namespace,
					"name", nc.Name,
					"component", appsv1alpha1.StoragedComponentType,
				)
			}
		}
	}

	return isReady, nil
}

func NebulaClusterReadyFuncForReplicas(graphdReplicas, metadReplicas, storagedReplicas int) NebulaClusterReadyFunc {
	return func(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (isReady bool, err error) {
		defer func() {
			if !isReady {
				klog.InfoS("Waiting for NebulaCluster to be ready but replicas not expected",
					"graphdReplicas", int(*nc.Spec.Graphd.Replicas),
					"graphdReplicasExpected", graphdReplicas,
					"metadReplicas", *nc.Spec.Metad.Replicas,
					"metadReplicasExpected", metadReplicas,
					"storagedReplicas", *nc.Spec.Storaged.Replicas,
					"storagedReplicasExpected", storagedReplicas,
				)
			}
		}()

		return int(*nc.Spec.Graphd.Replicas) == graphdReplicas &&
			int(*nc.Spec.Metad.Replicas) == metadReplicas &&
			int(*nc.Spec.Storaged.Replicas) == storagedReplicas, nil
	}
}

func isComponentStatusExpected(
	componentStatus *appsv1alpha1.ComponentStatus,
	componentSpec *appsv1alpha1.ComponentSpec,
	logKeysAndValues ...any,
) bool {
	isExpected := true
	if componentStatus.Version != componentSpec.Version {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Version not expected",
			append(
				logKeysAndValues,
				"expected", componentSpec.Version,
				"current", componentStatus.Version,
			)...,
		)
	}

	if componentStatus.Phase != appsv1alpha1.RunningPhase {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Phase is not Running",
			append(
				logKeysAndValues,
				"current", componentStatus.Phase,
			)...,
		)
	}

	if componentStatus.Workload.ReadyReplicas != *componentSpec.Replicas {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.ReadyReplicas not expected",
			append(
				logKeysAndValues,
				"expected", componentStatus.Workload.ReadyReplicas,
				"current", *componentSpec.Replicas,
			)...,
		)
	}

	if componentStatus.Workload.Replicas != *componentSpec.Replicas {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.Replicas not expected",
			append(
				logKeysAndValues,
				"expected", componentStatus.Workload.Replicas,
				"current", *componentSpec.Replicas,
			)...,
		)
	}

	if componentStatus.Workload.UpdatedReplicas != *componentSpec.Replicas {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.UpdatedReplicas not expected",
			append(
				logKeysAndValues,
				"expected", componentStatus.Workload.UpdatedReplicas,
				"current", *componentSpec.Replicas,
			)...,
		)
	}

	if componentStatus.Workload.CurrentReplicas != *componentSpec.Replicas {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.CurrentReplicas not expected",
			append(
				logKeysAndValues,
				"expected", componentStatus.Workload.CurrentReplicas,
				"current", *componentSpec.Replicas,
			)...,
		)
	}

	if componentStatus.Workload.AvailableReplicas != *componentSpec.Replicas {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.AvailableReplicas not expected",
			append(
				logKeysAndValues,
				"expected", componentStatus.Workload.AvailableReplicas,
				"current", *componentSpec.Replicas,
			)...,
		)
	}

	if componentStatus.Workload.UpdateRevision != componentStatus.Workload.CurrentRevision {
		isExpected = false
		klog.InfoS("Waiting for NebulaCluster to be ready but Workload.CurrentRevision not equal Workload.UpdateRevision",
			append(
				logKeysAndValues,
				"UpdateRevision", componentStatus.Workload.UpdateRevision,
				"CurrentRevision", componentStatus.Workload.CurrentRevision,
			)...,
		)
	}

	return isExpected
}
