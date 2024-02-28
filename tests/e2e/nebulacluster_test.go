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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

type (
	ncTestCase struct {
		Name                 string
		Labels               map[string]string
		DefaultNCOptions     []envfuncsext.NebulaClusterOption
		InstallNCOptions     []envfuncsext.NebulaClusterOption
		InstallWaitNCOptions []envfuncsext.NebulaClusterOption
		LoadLDBC             bool
		UpgradeCases         []ncTestUpgradeCase
		BackupCases          []ncBackupCase
	}

	ncTestUpgradeCase struct {
		Name                 string
		UpgradeFunc          features.Func // Customize the upgrade function, otherwise use the default upgrade with UpgradeNCOptions.
		UpgradeNCOptions     []envfuncsext.NebulaClusterOption
		UpgradeWaitNCOptions []envfuncsext.NebulaClusterOption
	}

	ncBackupCase struct {
		Name                 string
		BackupInstallOptions envfuncsext.NebulaBackupInstallOptions
		BackupUpdateOptions  map[string]any
		Incremental          bool
	}
)

func TestNebulaCluster(t *testing.T) {
	var ncTestCases []ncTestCase
	ncTestCases = append(ncTestCases, testCasesBasic...)
	ncTestCases = append(ncTestCases, testCasesCustomConfig...)
	ncTestCases = append(ncTestCases, testCasesTools...)
	ncTestCases = append(ncTestCases, testCasesZone...)
	ncTestCases = append(ncTestCases, testCasesPV...)
	ncTestCases = append(ncTestCases, testCasesK8s...)
	ncTestCases = append(ncTestCases, testCasesBackup...)

	defaultNebulaClusterHelmArgs := getDefaultNebulaClusterHelmArgs()

	testFeatures := make([]features.Feature, 0, len(ncTestCases))
	for caseIdx := range ncTestCases {
		caseIdx := caseIdx
		tc := ncTestCases[caseIdx]

		namespace := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)
		name := envconf.RandomName(fmt.Sprintf("e2e-nc-%d", caseIdx), 32)

		feature := features.New(tc.Name)

		for key, value := range tc.Labels {
			feature.WithLabel(key, value)
		}

		feature.Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			ctx, err = envfuncs.CreateNamespace(namespace)(ctx, cfg)
			if err != nil {
				t.Errorf("failed to create namespace %v", err)
			}

			ctx, err = envfuncsext.CreateObject(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ImagePullSecretName,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: config.C.DockerConfigJsonSecret,
					},
				},
			)(ctx, cfg)
			if err != nil {
				t.Errorf("failed to create secret %v", err)
			}

			return ctx
		})

		feature.Assess("Install NebulaCluster",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).InfoS("Install NebulaCluster", "namespace", namespace, "name", name)

				var err error

				opts := []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithChart(config.C.NebulaGraph.ChartPath),
						helm.WithNamespace(namespace),
						helm.WithName(name),
						helm.WithArgs(defaultNebulaClusterHelmArgs...),
						helm.WithArgs("--set", fmt.Sprintf("nameOverride=%s", name)),
					),
				}
				opts = append(opts, tc.DefaultNCOptions...)
				opts = append(opts, tc.InstallNCOptions...)

				ctx, err = envfuncsext.InstallNebulaCluster(opts...)(ctx, cfg)
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

		if tc.UpgradeCases != nil {
			for upgradeCaseIdx := range tc.UpgradeCases {
				upgradeCase := tc.UpgradeCases[upgradeCaseIdx]
				upgradeFunc := upgradeCase.UpgradeFunc
				if upgradeFunc == nil {
					upgradeFunc = func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
						klog.V(4).InfoS("Upgrade NebulaCluster", "namespace", namespace, "name", name)

						var err error

						opts := []envfuncsext.NebulaClusterOption{
							envfuncsext.WithNebulaClusterHelmRawOptions(
								helm.WithChart(config.C.NebulaGraph.ChartPath),
								helm.WithNamespace(namespace),
								helm.WithName(name),
								helm.WithArgs(defaultNebulaClusterHelmArgs...),
								helm.WithArgs("--set", fmt.Sprintf("nameOverride=%s", name)),
							),
						}
						opts = append(opts, tc.DefaultNCOptions...)
						opts = append(opts, upgradeCase.UpgradeNCOptions...)

						ctx, err = envfuncsext.UpgradeNebulaCluster(opts...)(ctx, cfg)
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
								wait.WithTimeout(time.Minute*12),
							),
						}, upgradeCase.UpgradeWaitNCOptions...)...)(ctx, cfg)
						if err != nil {
							t.Errorf("failed waiting for NebulaCluster to be ready %v", err)
						}
						return ctx
					},
				)
			}
		}

		if tc.BackupCases != nil {
			for backupCaseIdx := range tc.BackupCases {
				backupCase := tc.BackupCases[backupCaseIdx]

				err := setNebulaBackupSpecs(&backupCase.BackupInstallOptions.Spec, backupCase.BackupUpdateOptions)
				if err != nil {
					t.Errorf("failed to modify backup options for cluster [%v/%v]", namespace, name)
				}

				feature.Assess(fmt.Sprintf("Creating cloud provider secrets for testcase %v", backupCase.Name), func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					nsToUse := namespace
					if backupCase.BackupInstallOptions.Namespace != "" {
						nsToUse = backupCase.BackupInstallOptions.Namespace
					}

					cloudStorage, objectMeta, data := getCloudStorageSecretData(backupCase, nsToUse)

					klog.V(4).Infof("Creating %v secret for test case", cloudStorage)
					ctx, err := envfuncsext.CreateObject(
						&corev1.Secret{
							ObjectMeta: objectMeta,
							Type:       corev1.SecretTypeOpaque,
							Data:       data,
						},
					)(ctx, cfg)
					if err != nil {
						t.Errorf("failed to create %v secret: %v", cloudStorage, err)
					}
					klog.V(4).Infof("%v secret created successfully", cloudStorage)

					return ctx
				})

				if backupCase.BackupInstallOptions.Namespace != "" && backupCase.BackupInstallOptions.Namespace != namespace {
					feature.Assess("Creating needed image pull secret for cross namespace backup", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

						ctx, err := envfuncsext.CreateObject(
							&corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      ImagePullSecretName,
									Namespace: backupCase.BackupInstallOptions.Namespace,
								},
								Type: corev1.SecretTypeDockerConfigJson,
								Data: map[string][]byte{
									corev1.DockerConfigJsonKey: config.C.DockerConfigJsonSecret,
								},
							},
						)(ctx, cfg)
						if err != nil {
							t.Errorf("failed to create image pull secret %v", err)
						}

						return ctx
					})
				}

				feature.Assess(fmt.Sprintf("Creating base backup for %s for NebulaCluster", backupCase.Name), getNBCreateFunction(backupCase, false, name))

				feature.Assess(fmt.Sprintf("Wait for backup to be complete after %s", backupCase.Name), getNBWaitFunction(false, name))

				if backupCase.Incremental {
					feature.Assess(fmt.Sprintf("Creating Incremental backup for case %s for NebulaCluster", backupCase.Name), getNBCreateFunction(backupCase, true, name))

					feature.Assess(fmt.Sprintf("Wait for backup to be complete after incremental backup for %s", backupCase.Name), getNBWaitFunction(true, name))
				}

				feature.Assess(fmt.Sprintf("Delete backup after %s", backupCase.Name),
					func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
						klog.V(4).InfoS("Deleting backup for NebulaCluster", "cluster namespace", namespace, "cluster name", name)

						var err error
						if backupCase.Incremental {
							ctx, err = envfuncsext.DeleteNebulaBackup(true)(ctx, cfg)
							if err != nil {
								t.Errorf("Deleting incremental backup for NebulaCluster failed: %v", err)
							}
						}

						ctx, err = envfuncsext.DeleteNebulaBackup(false)(ctx, cfg)
						if err != nil {
							t.Errorf("Deleting base backup for NebulaCluster failed: %v", err)
						}
						return ctx
					},
				)

				if pointer.BoolDeref(backupCase.BackupInstallOptions.Spec.AutoRemoveFinished, false) {
					feature.Assess(fmt.Sprintf("Check auto delete after %s", backupCase.Name),
						func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
							klog.V(4).Info("Auto delete has been enabled. Checking if backup job is deleted for NebulaCluster", "cluster namespace", namespace, "cluster name", name)

							var err error
							if backupCase.Incremental {
								ctx, err = envfuncsext.WaitForCleanBackup(
									true,
									envfuncsext.WithNebulaBackupWaitOptions(
										wait.WithInterval(time.Second*5),
										wait.WithTimeout(time.Minute*16),
									),
								)(ctx, cfg)
								if err != nil {
									t.Errorf("error checking base backup job deletion or backup still exists: %v", err)
								} else {
									klog.Info("checking incremental backup job deletion successful. Nebulabackup no longer exists.")
								}
							}

							ctx, err = envfuncsext.WaitForCleanBackup(
								false,
								envfuncsext.WithNebulaBackupWaitOptions(
									wait.WithInterval(time.Second*5),
									wait.WithTimeout(time.Minute*16),
								),
							)(ctx, cfg)
							if err != nil {
								t.Errorf("error checking base backup job deletion or backup still exists: %v", err)
							} else {
								klog.Info("checking base backup job deletion successful. Nebulabackup no longer exists.")
							}
							return ctx
						},
					)
				}

				if backupCase.BackupInstallOptions.Namespace != "" && backupCase.BackupInstallOptions.Namespace != namespace {
					feature.Assess("Deleteing image pull secret for cross namespace backup", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

						ctx, err := envfuncsext.DeleteObject(
							&corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      ImagePullSecretName,
									Namespace: backupCase.BackupInstallOptions.Namespace,
								},
								Type: corev1.SecretTypeDockerConfigJson,
								Data: map[string][]byte{
									corev1.DockerConfigJsonKey: config.C.DockerConfigJsonSecret,
								},
							},
						)(ctx, cfg)
						if err != nil {
							t.Errorf("failed to create image pull secret %v", err)
						}

						return ctx
					})
				}

				feature.Assess(fmt.Sprintf("Deleting cloud provider secrets for testcase %v", backupCase.Name), func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					nsToUse := namespace
					if backupCase.BackupInstallOptions.Namespace != "" {
						nsToUse = backupCase.BackupInstallOptions.Namespace
					}

					cloudStorage, objectMeta, data := getCloudStorageSecretData(backupCase, nsToUse)

					klog.V(4).Infof("Deleting %v secret for test case", cloudStorage)
					ctx, err := envfuncsext.DeleteObject(
						&corev1.Secret{
							ObjectMeta: objectMeta,
							Type:       corev1.SecretTypeOpaque,
							Data:       data,
						},
					)(ctx, cfg)
					if err != nil {
						t.Errorf("failed to delete %v secret: %v", cloudStorage, err)
					}
					klog.V(4).Infof("%v secret deleted successfully", cloudStorage)

					return ctx
				})
			}
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
			if t.Failed() {
				klog.V(4).Info("Skipping teardown due to test failure.")
			} else {

				var err error
				ctx, err = envfuncs.DeleteNamespace(namespace)(ctx, cfg)
				if err != nil {
					t.Errorf("failed to delete namespace %v", err)
				}
			}
			return ctx
		})

		testFeatures = append(testFeatures, feature.Feature())
	}

	_ = testEnv.TestInParallel(t, testFeatures...)
}

func getDefaultNebulaClusterHelmArgs() []string {
	var args = []string{
		"--set", fmt.Sprintf("imagePullSecrets[0].name=%s", ImagePullSecretName),
	}

	if config.C.NebulaGraph.Version != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.version=%s", config.C.NebulaGraph.Version))
	}
	if config.C.NebulaGraph.AgentImage != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.agent.image=%s", config.C.NebulaGraph.AgentImage))
	}
	if config.C.NebulaGraph.AgentVersion != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.agent.version=%s", config.C.NebulaGraph.AgentVersion))
	}
	if config.C.NebulaGraph.GraphdImage != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.graphd.image=%s", config.C.NebulaGraph.GraphdImage))
	}
	if config.C.NebulaGraph.MetadImage != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.metad.image=%s", config.C.NebulaGraph.MetadImage))
	}
	if config.C.NebulaGraph.StoragedImage != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.storaged.image=%s", config.C.NebulaGraph.StoragedImage))
	}
	if config.C.NebulaGraph.LicenseManagerURL != "" {
		args = append(args, "--set", fmt.Sprintf("nebula.metad.licenseManagerURL=%s", config.C.NebulaGraph.LicenseManagerURL))
	}
	return args
}

func setNebulaBackupSpecs(backupSpec *appsv1alpha1.BackupSpec, backupUpdateOptions map[string]any) error {
	for option, value := range backupUpdateOptions {
		switch option {
		case "autoRemoveFinished":
			backupFinished := value.(*bool)
			backupSpec.AutoRemoveFinished = backupFinished
		case "cleanBackupData":
			cleanBackupData := value.(*bool)
			backupSpec.CleanBackupData = cleanBackupData
		case "storageProvider":
			storageProvider := value.(appsv1alpha1.StorageProvider)
			backupSpec.Config.StorageProvider = storageProvider
		default:
			return fmt.Errorf("error setting backup configs. Invalid option: %v", option)
		}
	}
	return nil
}

func getNBCreateFunction(backupCase ncBackupCase, incremental bool, clusterName string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		clusterNamespace := cfg.Namespace()
		klog.V(4).InfoS("Backup NebulaCluster", "namespace", clusterNamespace, "name", clusterName)

		backupCase.BackupInstallOptions.Spec.Config.ClusterName = clusterName
		backupCase.BackupInstallOptions.Spec.Config.ClusterNamespace = &clusterNamespace

		if incremental {
			backupContextValue := envfuncsext.GetNebulaBackupCtxValue(false, ctx)
			klog.Infof("Incremental backup detected. Base backup name: %v", backupContextValue.BackupFileName)
			backupCase.BackupInstallOptions.Name = fmt.Sprintf("%v-incr", backupCase.BackupInstallOptions.Name)
			backupCase.BackupInstallOptions.Spec.Config.BaseBackupName = &backupContextValue.BackupFileName
		}

		ctx, err := envfuncsext.DeployNebulaBackup(incremental, backupCase.BackupInstallOptions)(ctx, cfg)
		if err != nil {
			t.Errorf("failed to backup NebulaCluster %v", err)
		}
		return ctx
	}
}

func getNBWaitFunction(incremental bool, clusterName string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		klog.V(4).InfoS("Waiting for backup for NebulaCluster to be complete", "cluster namespace", cfg.Namespace(), "cluster name", clusterName)

		ctx, err := envfuncsext.WaitNebulaBackupFinished(
			incremental,
			envfuncsext.WithNebulaBackupWaitOptions(
				wait.WithInterval(time.Second*5),
				wait.WithTimeout(time.Minute*10),
			),
		)(ctx, cfg)
		if err != nil {
			t.Errorf("failed waiting for backup for NebulaCluster to be complete: %v", err)
		}

		return ctx
	}
}

func getCloudStorageSecretData(backupCase ncBackupCase, namespace string) (string, metav1.ObjectMeta, map[string][]byte) {
	var cloudStorage string
	var objectMeta metav1.ObjectMeta
	var data map[string][]byte

	if backupCase.BackupInstallOptions.Spec.Config.S3 != nil {
		cloudStorage = "S3"
		objectMeta = metav1.ObjectMeta{
			Name:      AWSSecretName,
			Namespace: namespace,
		}
		data = map[string][]byte{
			"access_key": config.C.AWSAccessKey,
			"secret_key": config.C.AWSSecretKey,
		}
	} else if backupCase.BackupInstallOptions.Spec.Config.GS != nil {
		cloudStorage = "GS"
		objectMeta = metav1.ObjectMeta{
			Name:      GSSecretName,
			Namespace: namespace,
		}
		data = map[string][]byte{
			"credentials": config.C.GSSecret,
		}
	}

	return cloudStorage, objectMeta, data
}
