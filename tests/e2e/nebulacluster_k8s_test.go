package e2e

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelCategoryK8s = "k8s"

	LabelGroupK8sEnv               = "env"
	LabelGroupK8sAnnotations       = "annotations"
	LabelGroupK8sLabels            = "labels"
	LabelGroupK8sNodeSelector      = "nodeSelector"
	LabelGroupK8sAffinity          = "affinity"
	LabelGroupK8sTolerations       = "tolerations"
	LabelGroupK8sInitContainers    = "initContainers"
	LabelGroupK8sSidecarContainers = "sidecarContainers"
	LabelGroupK8sVolumes           = "volumes"
	LabelGroupK8sReadinessProbe    = "readinessProbe"
	LabelGroupK8sLivenessProbe     = "livenessProbe"
)

var testCasesK8s []ncTestCase

func init() {
	testCasesK8s = append(testCasesK8s, testCaseK8sEnv...)
	testCasesK8s = append(testCasesK8s, testCaseK8sAnnotations...)
	testCasesK8s = append(testCasesK8s, testCaseK8sLabels...)
	testCasesK8s = append(testCasesK8s, testCaseK8sNodeSelector...)
	testCasesK8s = append(testCasesK8s, testCaseK8sAffinity...)
	testCasesK8s = append(testCasesK8s, testCaseK8sTolerations...)
	testCasesK8s = append(testCasesK8s, testCaseK8sInitContainers...)
	testCasesK8s = append(testCasesK8s, testCaseK8sSidecarContainers...)
	testCasesK8s = append(testCasesK8s, testCaseK8sVolumes...)
	testCasesK8s = append(testCasesK8s, testCaseK8sReadinessProbe...)
	testCasesK8s = append(testCasesK8s, testCaseK8sLivenessProbe...)
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

var testCaseK8sNodeSelector = []ncTestCase{
	{
		Name: "k8s nodeSelector with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sNodeSelector,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components nodeSelector",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.nodeSelector.kubernetes\\.io/os=linux",
							"--set", "nebula.storaged.nodeSelector.kubernetes\\.io/os=linux",
							"--set", "nebula.metad.nodeSelector.kubernetes\\.io/os=linux"),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"NodeSelector": e2ematcher.DeepEqual(map[string]string{
										"kubernetes.io/os": "linux",
									}),
								},
								"Storaged": map[string]any{
									"NodeSelector": e2ematcher.DeepEqual(map[string]string{
										"kubernetes.io/os": "linux",
									}),
								},
								"Metad": map[string]any{
									"NodeSelector": e2ematcher.DeepEqual(map[string]string{
										"kubernetes.io/os": "linux",
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

var testCaseK8sAffinity = []ncTestCase{
	{
		Name: "k8s affinity with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sAffinity,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components affinity",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight=1",
							"--set", "nebula.graphd.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels.app\\.kubernetes\\.io/component=graphd",
							"--set", "nebula.graphd.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey=topology.kubernetes.io/zone",
							"--set", "nebula.metad.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight=1",
							"--set", "nebula.metad.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels.app\\.kubernetes\\.io/component=metad",
							"--set", "nebula.metad.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey=topology.kubernetes.io/zone",
							"--set", "nebula.storaged.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight=1",
							"--set", "nebula.storaged.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchLabels.app\\.kubernetes\\.io/component=storaged",
							"--set", "nebula.storaged.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey=topology.kubernetes.io/zone",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Affinity": e2ematcher.DeepEqual(corev1.Affinity{
										PodAffinity: &corev1.PodAffinity{
											PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
												{
													Weight: 1,
													PodAffinityTerm: corev1.PodAffinityTerm{
														LabelSelector: &metav1.LabelSelector{
															MatchLabels: map[string]string{
																"app.kubernetes.io/component": "graphd",
															},
														},
														TopologyKey: "topology.kubernetes.io/zone",
													},
												},
											},
										},
									}),
								},
								"Metad": map[string]any{
									"Affinity": e2ematcher.DeepEqual(corev1.Affinity{
										PodAffinity: &corev1.PodAffinity{
											PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
												{
													Weight: 1,
													PodAffinityTerm: corev1.PodAffinityTerm{
														LabelSelector: &metav1.LabelSelector{
															MatchLabels: map[string]string{
																"app.kubernetes.io/component": "metad",
															},
														},
														TopologyKey: "topology.kubernetes.io/zone",
													},
												},
											},
										},
									}),
								},
								"Storaged": map[string]any{
									"Affinity": e2ematcher.DeepEqual(corev1.Affinity{
										PodAffinity: &corev1.PodAffinity{
											PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
												{
													Weight: 1,
													PodAffinityTerm: corev1.PodAffinityTerm{
														LabelSelector: &metav1.LabelSelector{
															MatchLabels: map[string]string{
																"app.kubernetes.io/component": "storaged",
															},
														},
														TopologyKey: "topology.kubernetes.io/zone",
													},
												},
											},
										},
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

var testCaseK8sTolerations = []ncTestCase{
	{
		Name: "k8s tolerations with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sTolerations,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components tolerations",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.tolerations[0].key=test-key",
							"--set", "nebula.graphd.tolerations[0].operator=Equal",
							"--set", "nebula.graphd.tolerations[0].value=test-value",
							"--set", "nebula.graphd.tolerations[0].effect=NoSchedule",
							"--set", "nebula.storaged.tolerations[0].key=test-key",
							"--set", "nebula.storaged.tolerations[0].operator=Equal",
							"--set", "nebula.storaged.tolerations[0].value=test-value",
							"--set", "nebula.storaged.tolerations[0].effect=NoSchedule",
							"--set", "nebula.metad.tolerations[0].key=test-key",
							"--set", "nebula.metad.tolerations[0].operator=Equal",
							"--set", "nebula.metad.tolerations[0].value=test-value",
							"--set", "nebula.metad.tolerations[0].effect=NoSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Tolerations": e2ematcher.DeepEqual([]corev1.Toleration{
										{
											Key:      "test-key",
											Operator: corev1.TolerationOpEqual,
											Value:    "test-value",
											Effect:   corev1.TaintEffectNoSchedule,
										},
									}),
								},
								"Storaged": map[string]any{
									"Tolerations": e2ematcher.DeepEqual([]corev1.Toleration{
										{
											Key:      "test-key",
											Operator: corev1.TolerationOpEqual,
											Value:    "test-value",
											Effect:   corev1.TaintEffectNoSchedule,
										},
									}),
								},
								"Metad": map[string]any{
									"Tolerations": e2ematcher.DeepEqual([]corev1.Toleration{
										{
											Key:      "test-key",
											Operator: corev1.TolerationOpEqual,
											Value:    "test-value",
											Effect:   corev1.TaintEffectNoSchedule,
										},
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

var testCaseK8sInitContainers = []ncTestCase{
	{
		Name: "k8s initContainers with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sInitContainers,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components initContainers",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.initContainers[0].name=init-container",
							"--set", "nebula.graphd.initContainers[0].image=busybox",
							"--set", "nebula.graphd.initContainers[0].command[0]=sh",
							"--set", "nebula.storaged.initContainers[0].name=init-container",
							"--set", "nebula.storaged.initContainers[0].image=busybox",
							"--set", "nebula.storaged.initContainers[0].command[0]=sh",
							"--set", "nebula.metad.initContainers[0].name=init-container",
							"--set", "nebula.metad.initContainers[0].image=busybox",
							"--set", "nebula.metad.initContainers[0].command[0]=sh",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"InitContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "init-container",
											Image:   "busybox",
											Command: []string{"sh"},
										},
									}),
								},
								"Storaged": map[string]any{
									"InitContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "init-container",
											Image:   "busybox",
											Command: []string{"sh"},
										},
									}),
								},
								"Metad": map[string]any{
									"InitContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "init-container",
											Image:   "busybox",
											Command: []string{"sh"},
										},
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

var testCaseK8sSidecarContainers = []ncTestCase{
	{
		Name: "k8s sidecarContainers with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sSidecarContainers,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components sidecarContainers",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.sidecarContainers[0].name=sidecar-container",
							"--set", "nebula.graphd.sidecarContainers[0].image=busybox",
							"--set", "nebula.graphd.sidecarContainers[0].command[0]=sleep",
							"--set", "nebula.graphd.sidecarContainers[0].command[1]=infinity",
							"--set", "nebula.storaged.sidecarContainers[0].name=sidecar-container",
							"--set", "nebula.storaged.sidecarContainers[0].image=busybox",
							"--set", "nebula.storaged.sidecarContainers[0].command[0]=sleep",
							"--set", "nebula.storaged.sidecarContainers[0].command[1]=infinity",
							"--set", "nebula.metad.sidecarContainers[0].name=sidecar-container",
							"--set", "nebula.metad.sidecarContainers[0].image=busybox",
							"--set", "nebula.metad.sidecarContainers[0].command[0]=sleep",
							"--set", "nebula.metad.sidecarContainers[0].command[1]=infinity",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"SidecarContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "sidecar-container",
											Image:   "busybox",
											Command: []string{"sleep", "infinity"},
										},
									}),
								},
								"Storaged": map[string]any{
									"SidecarContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "sidecar-container",
											Image:   "busybox",
											Command: []string{"sleep", "infinity"},
										},
									}),
								},
								"Metad": map[string]any{
									"SidecarContainers": e2ematcher.DeepEqual([]corev1.Container{
										{
											Name:    "sidecar-container",
											Image:   "busybox",
											Command: []string{"sleep", "infinity"},
										},
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

var testCaseK8sReadinessProbe = []ncTestCase{
	{
		Name: "k8s readinessProbe with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sReadinessProbe,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components readinessProbe",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.readinessProbe.httpGet.path=/status",
							"--set", "nebula.graphd.readinessProbe.httpGet.port=19669",
							"--set", "nebula.graphd.readinessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.graphd.readinessProbe.failureThreshold=3",
							"--set", "nebula.graphd.readinessProbe.periodSeconds=10",
							"--set", "nebula.graphd.readinessProbe.timeoutSeconds=1",
							"--set", "nebula.graphd.readinessProbe.successThreshold=1",
							"--set", "nebula.storaged.readinessProbe.httpGet.path=/status",
							"--set", "nebula.storaged.readinessProbe.httpGet.port=19779",
							"--set", "nebula.storaged.readinessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.storaged.readinessProbe.failureThreshold=3",
							"--set", "nebula.storaged.readinessProbe.periodSeconds=10",
							"--set", "nebula.storaged.readinessProbe.timeoutSeconds=1",
							"--set", "nebula.storaged.readinessProbe.successThreshold=1",
							"--set", "nebula.metad.readinessProbe.httpGet.path=/status",
							"--set", "nebula.metad.readinessProbe.httpGet.port=19559",
							"--set", "nebula.metad.readinessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.metad.readinessProbe.failureThreshold=3",
							"--set", "nebula.metad.readinessProbe.periodSeconds=10",
							"--set", "nebula.metad.readinessProbe.timeoutSeconds=1",
							"--set", "nebula.metad.readinessProbe.successThreshold=1",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"ReadinessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19669),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
									}),
								},
								"Storaged": map[string]any{
									"ReadinessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19779),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
									}),
								},
								"Metad": map[string]any{
									"ReadinessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19559),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
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

var testCaseK8sLivenessProbe = []ncTestCase{
	{
		Name: "k8s livenessProbe with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sLivenessProbe,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components livenessProbe",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.livenessProbe.httpGet.path=/status",
							"--set", "nebula.graphd.livenessProbe.httpGet.port=19669",
							"--set", "nebula.graphd.livenessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.graphd.livenessProbe.failureThreshold=3",
							"--set", "nebula.graphd.livenessProbe.periodSeconds=10",
							"--set", "nebula.graphd.livenessProbe.timeoutSeconds=1",
							"--set", "nebula.graphd.livenessProbe.successThreshold=1",
							"--set", "nebula.storaged.livenessProbe.httpGet.path=/status",
							"--set", "nebula.storaged.livenessProbe.httpGet.port=19779",
							"--set", "nebula.storaged.livenessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.storaged.livenessProbe.failureThreshold=3",
							"--set", "nebula.storaged.livenessProbe.periodSeconds=10",
							"--set", "nebula.storaged.livenessProbe.timeoutSeconds=1",
							"--set", "nebula.storaged.livenessProbe.successThreshold=1",
							"--set", "nebula.metad.livenessProbe.httpGet.path=/status",
							"--set", "nebula.metad.livenessProbe.httpGet.port=19559",
							"--set", "nebula.metad.livenessProbe.httpGet.scheme=HTTP",
							"--set", "nebula.metad.livenessProbe.failureThreshold=3",
							"--set", "nebula.metad.livenessProbe.periodSeconds=10",
							"--set", "nebula.metad.livenessProbe.timeoutSeconds=1",
							"--set", "nebula.metad.livenessProbe.successThreshold=1",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"LivenessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19669),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
									}),
								},
								"Storaged": map[string]any{
									"LivenessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19779),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
									}),
								},
								"Metad": map[string]any{
									"LivenessProbe": e2ematcher.DeepEqual(corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/status",
												Port:   intstr.FromInt(19559),
												Scheme: "HTTP",
											},
										},
										FailureThreshold: int32(3),
										PeriodSeconds:    int32(10),
										TimeoutSeconds:   int32(1),
										SuccessThreshold: int32(1),
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

var testCaseK8sVolumes = []ncTestCase{
	{
		Name: "k8s volumes with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryK8s,
			LabelKeyGroup:    LabelGroupK8sVolumes,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update components volumes",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.volumes[0].name=test-volume",
							"--set", "nebula.graphd.volumes[0].emptyDir.medium=Memory",
							"--set", "nebula.graphd.volumeMounts[0].name=test-volume",
							"--set", "nebula.graphd.volumeMounts[0].mountPath=/test",
							"--set", "nebula.storaged.volumes[0].name=test-volume",
							"--set", "nebula.storaged.volumes[0].emptyDir.medium=Memory",
							"--set", "nebula.storaged.volumeMounts[0].name=test-volume",
							"--set", "nebula.storaged.volumeMounts[0].mountPath=/test",
							"--set", "nebula.metad.volumes[0].name=test-volume",
							"--set", "nebula.metad.volumes[0].emptyDir.medium=Memory",
							"--set", "nebula.metad.volumeMounts[0].name=test-volume",
							"--set", "nebula.metad.volumeMounts[0].mountPath=/test",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Volumes": e2ematcher.DeepEqual([]corev1.Volume{
										{
											Name: "test-volume",
											VolumeSource: corev1.VolumeSource{
												EmptyDir: &corev1.EmptyDirVolumeSource{
													Medium: corev1.StorageMediumMemory,
												},
											},
										},
									}),
									"VolumeMounts": e2ematcher.DeepEqual([]corev1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/test",
										},
									}),
								},
								"Storaged": map[string]any{
									"Volumes": e2ematcher.DeepEqual([]corev1.Volume{
										{
											Name: "test-volume",
											VolumeSource: corev1.VolumeSource{
												EmptyDir: &corev1.EmptyDirVolumeSource{
													Medium: corev1.StorageMediumMemory,
												},
											},
										},
									}),
									"VolumeMounts": e2ematcher.DeepEqual([]corev1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/test",
										},
									}),
								},
								"Metad": map[string]any{
									"Volumes": e2ematcher.DeepEqual([]corev1.Volume{
										{
											Name: "test-volume",
											VolumeSource: corev1.VolumeSource{
												EmptyDir: &corev1.EmptyDirVolumeSource{
													Medium: corev1.StorageMediumMemory,
												},
											},
										},
									}),
									"VolumeMounts": e2ematcher.DeepEqual([]corev1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/test",
										},
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
