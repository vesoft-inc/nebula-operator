package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelCustomConfig        = "custom config"
	LabelCustomConfigStatic  = "static"
	LabelCustomConfigDynamic = "dynamic"
	LabelCustomConfigPort    = "port"
)

var testCasesCustomConfig []ncTestCase

func init() {
	testCasesCustomConfig = append(testCasesCustomConfig, testCasesCustomConfigStatic...)
	testCasesCustomConfig = append(testCasesCustomConfig, testCasesCustomConfigDynamic...)
	testCasesCustomConfig = append(testCasesCustomConfig, testCasesCustomConfigPort...)
}

// test cases about static custom config
var testCasesCustomConfigStatic = []ncTestCase{
	{
		Name: "custom config for static",
		Labels: map[string]string{
			LabelKeyCategory: LabelCustomConfig,
			LabelKeyGroup:    LabelCustomConfigStatic,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
						},
						"Storaged": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update configs",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set-string", "nebula.graphd.config.max_sessions_per_ip_per_user=100",
							"--set-string", "nebula.metad.config.default_parts_num=30",
							"--set-string", "nebula.storaged.config.minimum_reserved_bytes=134217728",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"max_sessions_per_ip_per_user": e2ematcher.ValidatorEq("100"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"default_parts_num": e2ematcher.ValidatorEq("30"),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"minimum_reserved_bytes": e2ematcher.ValidatorEq("134217728"),
									},
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name: "restart pods",
				UpgradeFunc: func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					pods := corev1.PodList{}
					var err error
					ncCtxValue := envfuncsext.GetNebulaClusterCtxValue(ctx)
					labelSelector := fmt.Sprintf("app.kubernetes.io/cluster=%s,app.kubernetes.io/component in (graphd, metad, storaged)", ncCtxValue.Name)
					if err = cfg.Client().Resources().List(ctx, &pods, resources.WithLabelSelector(labelSelector)); err != nil {
						t.Errorf("failed to list pods %s %v", labelSelector, err)
					}
					for _, pod := range pods.Items {
						if err = cfg.Client().Resources().Delete(ctx, &pod); err != nil {
							t.Errorf("failed to delete pod %s/%s %v", pod.Namespace, pod.Name, err)
						}
					}
					return ctx
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"max_sessions_per_ip_per_user": e2ematcher.ValidatorEq("100"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"default_parts_num": e2ematcher.ValidatorEq("30"),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"minimum_reserved_bytes": e2ematcher.ValidatorEq("134217728"),
									},
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
		},
	},
}

// test cases about dynamic custom config
var testCasesCustomConfigDynamic = []ncTestCase{
	{
		Name: "custom config for dynamic",
		Labels: map[string]string{
			LabelKeyCategory: LabelCustomConfig,
			LabelKeyGroup:    LabelCustomConfigDynamic,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
						},
						"Storaged": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update configs",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set-string", "nebula.graphd.config.v=1",
							"--set-string", "nebula.metad.config.v=2",
							"--set-string", "nebula.storaged.config.v=3",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("2"),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("3"),
									},
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name: "restart pods",
				UpgradeFunc: func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					pods := corev1.PodList{}
					var err error
					ncCtxValue := envfuncsext.GetNebulaClusterCtxValue(ctx)
					labelSelector := fmt.Sprintf("app.kubernetes.io/cluster=%s,app.kubernetes.io/component in (graphd, metad, storaged)", ncCtxValue.Name)
					if err = cfg.Client().Resources().List(ctx, &pods, resources.WithLabelSelector(labelSelector)); err != nil {
						t.Errorf("failed to list pods %s %v", labelSelector, err)
					}
					for _, pod := range pods.Items {
						if err = cfg.Client().Resources().Delete(ctx, &pod); err != nil {
							t.Errorf("failed to delete pod %s/%s %v", pod.Namespace, pod.Name, err)
						}
					}
					return ctx
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("2"),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"v": e2ematcher.ValidatorEq("3"),
									},
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
		},
	},
}

// test cases about port custom config
var testCasesCustomConfigPort = []ncTestCase{
	{
		Name: "custom config thrift port and http port",
		Labels: map[string]string{
			LabelKeyCategory: LabelCustomConfig,
			LabelKeyGroup:    LabelCustomConfigPort,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs( // thrift 90x9, http 190x9
					"--set-string", "nebula.graphd.config.port=9069",
					"--set-string", "nebula.graphd.config.ws_http_port=19069",
					"--set-string", "nebula.metad.config.port=9059",
					"--set-string", "nebula.metad.config.ws_http_port=19059",
					"--set-string", "nebula.storaged.config.port=9079",
					"--set-string", "nebula.storaged.config.ws_http_port=19079",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
							"Config": map[string]any{
								"port":         e2ematcher.ValidatorEq("9069"),
								"ws_http_port": e2ematcher.ValidatorEq("19069"),
							},
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"Config": map[string]any{
								"port":         e2ematcher.ValidatorEq("9059"),
								"ws_http_port": e2ematcher.ValidatorEq("19059"),
							},
						},
						"Storaged": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"Config": map[string]any{
								"port":         e2ematcher.ValidatorEq("9079"),
								"ws_http_port": e2ematcher.ValidatorEq("19079"),
							},
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update http ports",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs( // http 190x9 => 191x9
							"--set-string", "nebula.graphd.config.port=9069",
							"--set-string", "nebula.graphd.config.ws_http_port=19169",
							"--set-string", "nebula.metad.config.port=9059",
							"--set-string", "nebula.metad.config.ws_http_port=19159",
							"--set-string", "nebula.storaged.config.port=9079",
							"--set-string", "nebula.storaged.config.ws_http_port=19179",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"port":         e2ematcher.ValidatorEq("9069"),
										"ws_http_port": e2ematcher.ValidatorEq("19169"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"port":         e2ematcher.ValidatorEq("9059"),
										"ws_http_port": e2ematcher.ValidatorEq("19159"),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"port":         e2ematcher.ValidatorEq("9079"),
										"ws_http_port": e2ematcher.ValidatorEq("19179"),
									},
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
		},
	},
}
