package e2e

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelCategoryK8s = "k8s"

	LabelGroupK8sEnv         = "env"
	LabelGroupK8sAnnotations = "annotations"
	LabelGroupK8sLabels      = "labels"
)

var testCasesK8s []ncTestCase

func init() {
	testCasesK8s = append(testCasesK8s, testCaseK8sEnv...)
	testCasesK8s = append(testCasesK8s, testCaseK8sAnnotations...)
	testCasesK8s = append(testCasesK8s, testCaseK8sLabels...)
}

var testCaseK8sEnv = []ncTestCase{
	{
		Name: "k8s env with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sEnv,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{}),
						},
						"Metad": map[string]any{
							"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{}),
						},
						"Storaged": map[string]any{
							"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{}),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components env",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.env[0].name=TEST_VAR1,nebula.graphd.env[0].value=TEST_VALUE1",
							"--set", "nebula.storaged.env[0].name=TEST_VAR2,nebula.storaged.env[0].value=TEST_VALUE2",
							"--set", "nebula.metad.env[0].name=TEST_VAR3,nebula.metad.env[0].value=TEST_VALUE3",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{
										{Name: "TEST_VAR1", Value: "TEST_VALUE1"},
									}),
								},
								"Storaged": map[string]any{
									"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{
										{Name: "TEST_VAR2", Value: "TEST_VALUE2"},
									}),
								},
								"Metad": map[string]any{
									"EnvVars": e2ematcher.DeepEqual([]corev1.EnvVar{
										{Name: "TEST_VAR3", Value: "TEST_VALUE3"},
									}),
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

var testCaseK8sAnnotations = []ncTestCase{
	{
		Name: "k8s annotations with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sAnnotations,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Annotations": e2ematcher.DeepEqual(map[string]string{}),
						},
						"Metad": map[string]any{
							"Annotations": e2ematcher.DeepEqual(map[string]string{}),
						},
						"Storaged": map[string]any{
							"Annotations": e2ematcher.DeepEqual(map[string]string{}),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components annotations",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.podAnnotations.key1=value1",
							"--set", "nebula.storaged.podAnnotations.key2=value2",
							"--set", "nebula.metad.podAnnotations.key3=value3"),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Annotations": e2ematcher.DeepEqual(map[string]string{
										"key1": "value1",
									}),
								},
								"Storaged": map[string]any{
									"Annotations": e2ematcher.DeepEqual(map[string]string{
										"key2": "value2",
									}),
								},
								"Metad": map[string]any{
									"Annotations": e2ematcher.DeepEqual(map[string]string{
										"key3": "value3",
									}),
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

var testCaseK8sLabels = []ncTestCase{
	{
		Name: "k8s labels with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sLabels,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Labels": e2ematcher.DeepEqual(map[string]string{}),
						},
						"Metad": map[string]any{
							"Labels": e2ematcher.DeepEqual(map[string]string{}),
						},
						"Storaged": map[string]any{
							"Labels": e2ematcher.DeepEqual(map[string]string{}),
						},
					},
				},
				),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components labels",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.podLabels.key1=value1",
							"--set", "nebula.storaged.podLabels.key2=value2",
							"--set", "nebula.metad.podLabels.key3=value3"),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Labels": e2ematcher.DeepEqual(map[string]string{
										"key1": "value1",
									}),
								},
								"Storaged": map[string]any{
									"Labels": e2ematcher.DeepEqual(map[string]string{
										"key2": "value2",
									}),
								},
								"Metad": map[string]any{
									"Labels": e2ematcher.DeepEqual(map[string]string{
										"key3": "value3",
									}),
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
