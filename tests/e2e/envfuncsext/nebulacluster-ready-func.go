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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
)

var defaultNebulaClusterReadyFuncs = []NebulaClusterReadyFunc{
	defaultNebulaClusterReadyFuncForStatus,
	defaultNebulaClusterReadyFuncForGraphd,
	defaultNebulaClusterReadyFuncForMetad,
	defaultNebulaClusterReadyFuncForStoraged,
	defaultNebulaClusterReadyFuncForAgent,
	defaultNebulaClusterReadyFuncForExporter,
	defaultNebulaClusterReadyFuncForConsole,
}

func DefaultNebulaClusterReadyFunc(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	for _, fn := range defaultNebulaClusterReadyFuncs {
		if isReady, err := fn(ctx, cfg, nc); !isReady || err != nil {
			return isReady, err
		}
	}
	return true, nil
}

func NebulaClusterReadyFuncForFields(ignoreValidationErrors bool, matchersMapping map[string]any) NebulaClusterReadyFunc {
	return func(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
		if err := e2ematcher.Struct(nc, matchersMapping); err != nil {
			if ignoreValidationErrors {
				klog.InfoS("Waiting for NebulaCluster to be ready but not expected", "err", err)
				return false, nil
			}

			klog.Error(err, "Waiting for NebulaCluster to be ready but not expected")
			return false, stderrors.New("check NebulaCluster")
		}
		return true, nil
	}
}

func defaultNebulaClusterReadyFuncForStatus(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
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

func defaultNebulaClusterReadyFuncForGraphd(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{ // Graphd StatefulSet checks
		if !isComponentStatefulSetExpected(ctx, cfg, nc.GraphdComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForMetad(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{ // Metad Resource checks
		if !isComponentStatefulSetExpected(ctx, cfg, nc.MetadComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForStoraged(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{ // Storaged Resource checks
		if !isComponentStatefulSetExpected(ctx, cfg, nc.StoragedComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForAgent(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func defaultNebulaClusterReadyFuncForExporter(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func defaultNebulaClusterReadyFuncForConsole(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func isComponentStatusExpected(
	componentStatus *appsv1alpha1.ComponentStatus,
	componentSpec *appsv1alpha1.ComponentSpec,
	logKeysAndValues ...any,
) bool {
	if err := e2ematcher.Struct(
		componentStatus,
		map[string]any{
			"Version": e2ematcher.ValidatorEq(componentSpec.Version),
			"Phase":   e2ematcher.ValidatorEq(appsv1alpha1.RunningPhase),
			"Workload": map[string]any{
				"ReadyReplicas":     e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"UpdatedReplicas":   e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"CurrentReplicas":   e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"AvailableReplicas": e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"CurrentRevision":   e2ematcher.ValidatorEq(componentStatus.Workload.UpdateRevision),
			},
		},
	); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but componentStatus not expected", logKeysAndValues...)
		return false
	}

	return true
}

func isComponentStatefulSetExpected(ctx context.Context, cfg *envconf.Config, component appsv1alpha1.NebulaClusterComponent) bool {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.GetName(),
			Namespace: component.GetNamespace(),
		},
	}

	if err := cfg.Client().Resources().Get(ctx, sts.Name, sts.Namespace, sts); err != nil {
		klog.InfoS("Check Component Resource but StatefulSet not found",
			"namespace", sts.Namespace,
			"name", sts.Name,
		)
		return false
	}

	componentContainerIdx := -1
	for i, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == component.ComponentType().String() {
			componentContainerIdx = i
			break
		}
	}

	if componentContainerIdx == -1 {
		klog.InfoS("Check Component Resource but container not found",
			"namespace", sts.Namespace,
			"name", sts.Name,
			"container", component.ComponentType().String(),
		)
		return false
	}

	klog.InfoS("Check Component Resource find container",
		"namespace", sts.Namespace,
		"name", sts.Name,
		"container", component.ComponentType().String(),
		"index", componentContainerIdx,
	)

	if err := e2ematcher.Struct(
		sts,
		map[string]any{
			"ObjectMeta": map[string]any{
				"Name":      e2ematcher.ValidatorEq(component.GetName()),
				"Namespace": e2ematcher.ValidatorEq(component.GetNamespace()),
			},
			"Spec": map[string]any{
				"Replicas": e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"Template": map[string]any{
					"Spec": map[string]any{
						"Containers": map[string]any{
							fmt.Sprint(componentContainerIdx): map[string]any{
								"Image":     e2ematcher.ValidatorEq(component.ComponentSpec().PodImage()),
								"Resources": e2ematcher.DeepEqual(*component.ComponentSpec().Resources()),
							},
						},
					},
				},
			},
		},
	); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but StatefulSet not expected",
			"namespace", sts.Namespace,
			"name", sts.Name,
		)
		return false
	}

	return true
}
