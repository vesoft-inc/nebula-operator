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

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2evalidator"
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

func NebulaClusterReadyFuncForFields(ignoreValidationErrors bool, rulesMapping map[string]e2evalidator.Rule) NebulaClusterReadyFunc {
	return func(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (isReady bool, err error) {
		if errMessages := e2evalidator.StructWithRules(nc, rulesMapping); len(errMessages) > 0 {
			if ignoreValidationErrors {
				klog.InfoS("Waiting for NebulaCluster to be ready but not expected", "errMessages", errMessages)
				return false, nil
			}

			klog.Error(nil, "Waiting for NebulaCluster to be ready but not expected", "errMessages", errMessages)
			return false, stderrors.New("check NebulaCluster")
		}
		return true, nil
	}
}

func defaultNebulaClusterReadyFuncForStatus(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
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

func defaultNebulaClusterReadyFuncForGraphd(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func defaultNebulaClusterReadyFuncForMetad(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func defaultNebulaClusterReadyFuncForStoraged(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
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
	if errMessages := e2evalidator.StructWithRules(
		componentStatus,
		map[string]e2evalidator.Rule{
			"Version":                    e2evalidator.Eq(componentSpec.Version),
			"Phase":                      e2evalidator.Eq(appsv1alpha1.RunningPhase),
			"Workload.ReadyReplicas":     e2evalidator.Eq(*componentSpec.Replicas),
			"Workload.UpdatedReplicas":   e2evalidator.Eq(*componentSpec.Replicas),
			"Workload.CurrentReplicas":   e2evalidator.Eq(*componentSpec.Replicas),
			"Workload.AvailableReplicas": e2evalidator.Eq(*componentSpec.Replicas),
			"Workload.CurrentRevision":   e2evalidator.Eq(componentStatus.Workload.UpdateRevision),
		},
	); len(errMessages) > 0 {
		klog.InfoS("Waiting for NebulaCluster to be ready but componentStatus not expected",
			append(
				logKeysAndValues,
				"errMessages", errMessages,
			)...,
		)
		return false
	}

	return true
}
