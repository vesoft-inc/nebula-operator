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

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

func TestNebulaClusterBasic(t *testing.T) {
	type tmpTestUpgradeCase struct {
		Name                 string
		UpgradeFunc          features.Func // Customize the upgrade function, otherwise use the default upgrade with UpgradeNCOptions.
		UpgradeNCOptions     []envfuncsext.NebulaClusterOption
		UpgradeWaitNCOptions []envfuncsext.NebulaClusterOption
	}
	type tmpTestCase struct {
		Name                 string
		InstallNCOptions     []envfuncsext.NebulaClusterOption
		InstallWaitNCOptions []envfuncsext.NebulaClusterOption
		UpgradeCases         []tmpTestUpgradeCase
	}

	testCases := []tmpTestCase{
		{
			Name: "default 2-3-3",
			InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
				envfuncsext.WithNebulaClusterReadyFuncs(
					envfuncsext.NebulaClusterReadyFuncForReplicas(2, 3, 3),
					envfuncsext.DefaultNebulaClusterReadyFunc,
				),
			},
			UpgradeCases: []tmpTestUpgradeCase{
				{
					Name:        "scale out [graphd, storaged]: 4-3-4",
					UpgradeFunc: nil,
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=4",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=4",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(4, 3, 4),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
				{
					Name:        "scale out  [graphd]: 5-3-4",
					UpgradeFunc: nil,
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=5",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=4",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(5, 3, 4),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
				{
					Name:        "scale out [storaged]: 5-3-5",
					UpgradeFunc: nil,
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=5",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=5",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(5, 3, 5),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
				{
					Name: "scale in [graphd, storaged]: 3-3-4",
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=3",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=4",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(3, 3, 4),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
				{
					Name: "scale in [storaged]: 3-3-3",
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=3",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=3",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(3, 3, 3),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
				{
					Name: "scale in[graphd]: 2-3-3",
					UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithArgs(
								"--set", "nebula.graphd.replicas=2",
								"--set", "nebula.metad.replicas=3",
								"--set", "nebula.storaged.replicas=3",
							),
						),
					},
					UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterReadyFuncs(
							envfuncsext.NebulaClusterReadyFuncForReplicas(2, 3, 3),
							envfuncsext.DefaultNebulaClusterReadyFunc,
						),
					},
				},
			},
		},
	}

	testFeatures := make([]features.Feature, 0, len(testCases))
	for caseIdx := range testCases {
		caseIdx := caseIdx
		tc := testCases[caseIdx]

		namespace := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)
		name := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)

		feature := features.New(fmt.Sprintf("Create NebulaCluster %s", tc.Name))

		feature.Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			ctx, err = envfuncs.CreateNamespace(namespace)(ctx, cfg)
			if err != nil {
				t.Errorf("failed to create namespace %v", err)
			}
			return ctx
		})

		feature.Assess("Install NebulaCluster",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).InfoS("Install NebulaCluster", "namespace", namespace, "name", name)

				var err error
				ctx, err = envfuncsext.InstallNebulaCluster(append([]envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithChart(config.C.NebulaGraph.ChartPath),
						helm.WithNamespace(namespace),
						helm.WithName(name),
						helm.WithArgs("--set", fmt.Sprintf("nameOverride=%s", name)),
					),
				}, tc.InstallNCOptions...)...)(ctx, cfg)
				if err != nil {
					t.Errorf("failed to install NebulaCluster %v", err)
				}
				return ctx
			},
		)

		feature.Assess("Wait NebulaCluster to be ready",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).InfoS("Waiting for NebulaCluster to be ready", "namespace", namespace, "name", name)

				var err error
				ctx, err = envfuncsext.WaitNebulaClusterReady(append([]envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterWaitOptions(
						wait.WithInterval(time.Second*5),
						wait.WithTimeout(time.Minute*5),
					),
				}, tc.InstallWaitNCOptions...)...)(ctx, cfg)
				if err != nil {
					t.Errorf("failed waiting for NebulaCluster to be ready %v", err)
				}
				return ctx
			},
		)

		for upgradeCaseIdx := range tc.UpgradeCases {
			upgradeCase := tc.UpgradeCases[upgradeCaseIdx]
			upgradeFunc := upgradeCase.UpgradeFunc
			if upgradeFunc == nil {
				upgradeFunc = func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					klog.V(4).InfoS("Upgrade NebulaCluster", "namespace", namespace, "name", name)

					var err error
					ctx, err = envfuncsext.UpgradeNebulaCluster(append([]envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterHelmRawOptions(
							helm.WithChart(config.C.NebulaGraph.ChartPath),
							helm.WithNamespace(namespace),
							helm.WithName(name),
							helm.WithArgs("--set", fmt.Sprintf("nameOverride=%s", name)),
						),
					}, upgradeCase.UpgradeNCOptions...)...)(ctx, cfg)
					if err != nil {
						t.Errorf("failed to upgrade NebulaCluster %v", err)
					}
					return ctx
				}
			}
			feature.Assess(fmt.Sprintf("Upgrade NebulaCluster %s", upgradeCase.Name), upgradeFunc)

			feature.Assess(fmt.Sprintf("Wait NebulaCluster to be ready after upgrade %s", upgradeCase.Name),
				func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					klog.V(4).InfoS("Waiting for NebulaCluster to be ready", "namespace", namespace, "name", name)

					var err error
					ctx, err = envfuncsext.WaitNebulaClusterReady(append([]envfuncsext.NebulaClusterOption{
						envfuncsext.WithNebulaClusterWaitOptions(
							wait.WithInterval(time.Second*5),
							wait.WithTimeout(time.Minute*5),
						),
					}, upgradeCase.UpgradeWaitNCOptions...)...)(ctx, cfg)
					if err != nil {
						t.Errorf("failed waiting for NebulaCluster to be ready %v", err)
					}
					return ctx
				},
			)
		}

		feature.Assess("Uninstall NebulaCluster",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).InfoS("Uninstall NebulaCluster", "namespace", namespace, "name", name)

				var err error
				ctx, err = envfuncsext.UninstallNebulaCluster(
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithNamespace(namespace),
						helm.WithName(name),
					),
				)(ctx, cfg)
				if err != nil {
					t.Errorf("failed to uninstall NebulaCluster %v", err)
				}
				return ctx
			},
		)

		feature.Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			ctx, err = envfuncs.DeleteNamespace(namespace)(ctx, cfg)
			if err != nil {
				t.Errorf("failed to delete namespace %v", err)
			}
			return ctx
		})

		testFeatures = append(testFeatures, feature.Feature())
	}

	_ = testEnv.TestInParallel(t, testFeatures...)
}
