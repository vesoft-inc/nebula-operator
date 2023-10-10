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

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

var ncGlobalTestCases []ncTestCase

func init() {
	ncGlobalTestCases = append(ncGlobalTestCases, testCasesBasic...)
}

type (
	ncTestCase struct {
		Name                 string
		Labels               map[string]string
		InstallNCOptions     []envfuncsext.NebulaClusterOption
		InstallWaitNCOptions []envfuncsext.NebulaClusterOption
		LoadLDBC             bool
		UpgradeCases         []ncTestUpgradeCase
	}

	ncTestUpgradeCase struct {
		Name                 string
		UpgradeFunc          features.Func // Customize the upgrade function, otherwise use the default upgrade with UpgradeNCOptions.
		UpgradeNCOptions     []envfuncsext.NebulaClusterOption
		UpgradeWaitNCOptions []envfuncsext.NebulaClusterOption
	}
)

func TestNebulaCluster(t *testing.T) {
	testFeatures := make([]features.Feature, 0, len(ncGlobalTestCases))
	for caseIdx := range ncGlobalTestCases {
		caseIdx := caseIdx
		tc := ncGlobalTestCases[caseIdx]

		namespace := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)
		name := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)

		feature := features.New(fmt.Sprintf("Create NebulaCluster %s", tc.Name))

		feature.WithLabel(LabelKeyCase, tc.Name)
		for key, value := range tc.Labels {
			feature.WithLabel(key, value)
		}

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

		if tc.LoadLDBC {
			feature.Assess("Load LDBC-SNB dataset",
				func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					klog.V(4).InfoS("Loading LDBC-SNB dataset", "namespace", namespace, "name", name)

					var err error

					ncCtxValue := envfuncsext.GetNebulaClusterCtxValue(ctx)
					nc := &appsv1alpha1.NebulaCluster{}
					if err = cfg.Client().Resources().Get(ctx, ncCtxValue.Name, ncCtxValue.Namespace, nc); err != nil {
						t.Errorf("failed to get NebulaCluster %v", err)
					}

					ctx, err = envfuncsext.ImportLDBC(
						envfuncsext.WithImporterName(nc.Name+"-import-ldbc"),
						envfuncsext.WithImporterNamespace(nc.Namespace),
						envfuncsext.WithImporterClientAddress(nc.GraphdComponent().GetConnAddress(appsv1alpha1.GraphdPortNameThrift)),
						envfuncsext.WithImporterWaitOptions(
							wait.WithInterval(time.Second*5),
							wait.WithTimeout(time.Minute*5),
						),
					)(ctx, cfg)
					if err != nil {
						t.Errorf("failed to create importer to load data %v", err)
					}
					return ctx
				},
			)
		}

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
