package e2e

import (
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelCategoryBasic  = "basic"
	LabelGroupScale     = "scale"
	LabelGroupVersion   = "version"
	LabelGroupResources = "resources"
)

var testCasesBasic []ncTestCase

func init() {
	testCasesBasic = append(testCasesBasic, testCasesBasicScale...)
	testCasesBasic = append(testCasesBasic, testCasesBasicVersion...)
	testCasesBasic = append(testCasesBasic, testCasesBasicResources...)
	testCasesBasic = append(testCasesBasic, testCasesBasicImage...)
	testCasesBasic = append(testCasesBasic, testCasesBasicVolume...)
}

// test cases about scale
var testCasesBasicScale = []ncTestCase{
	{
		Name: "scale with default values",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryBasic,
			LabelKeyGroup:    LabelGroupScale,
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
								},
							},
						}),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(5),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
								},
							},
						}),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(5),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(5),
								},
							},
						}),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
								},
							},
						}),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
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
			},
		},
	},
}

// test cases about version of graphd｜metad｜storaged
var testCasesBasicVersion = []ncTestCase{
	{
		Name: "update version",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryBasic,
			LabelKeyGroup:    LabelGroupVersion,
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
							"Version":  e2ematcher.ValidatorNe("latest"),
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"Version":  e2ematcher.ValidatorNe("latest"),
						},
						"Storaged": map[string]any{
							"Replicas":          e2ematcher.ValidatorEq(3),
							"Version":           e2ematcher.ValidatorNe("latest"),
							"EnableForceUpdate": e2ematcher.ValidatorEq(false),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update version",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.version=latest",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Version":  e2ematcher.ValidatorEq("latest"),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorEq("latest"),
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"Version":           e2ematcher.ValidatorEq("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(false),
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name:        "update version with scale",
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
									"Version":  e2ematcher.ValidatorNe("latest"),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorNe("latest"),
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(4),
									"Version":           e2ematcher.ValidatorNe("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(false),
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name:        "update version with scale and enableForceUpdate",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.version=latest",
							"--set", "nebula.graphd.replicas=2",
							"--set", "nebula.metad.replicas=3",
							"--set", "nebula.storaged.replicas=3",
							"--set", "nebula.enableForceUpdate=true",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Version":  e2ematcher.ValidatorEq("latest"),
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorEq("latest"),
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"Version":           e2ematcher.ValidatorEq("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(true),
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

// test cases about resources of graphd｜metad｜storaged
var testCasesBasicResources = []ncTestCase{
	{
		Name: "update resources",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryBasic,
			LabelKeyGroup:    LabelGroupResources,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set", "nebula.enablePVReclaim=true",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
					"Spec": map[string]any{
						"Metad": map[string]any{
							"Resources": e2ematcher.DeepEqual(
								corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"cpu":    resource.MustParse("500m"),
										"memory": resource.MustParse("500Mi"),
									},
									Limits: corev1.ResourceList{
										"cpu":    resource.MustParse("1"),
										"memory": resource.MustParse("1Gi"),
									},
								},
							),
						},
						"Storaged": map[string]any{
							"Resources": e2ematcher.DeepEqual(
								corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"cpu":    resource.MustParse("500m"),
										"memory": resource.MustParse("500Mi"),
									},
									Limits: corev1.ResourceList{
										"cpu":    resource.MustParse("1"),
										"memory": resource.MustParse("1Gi"),
									},
								},
							),
						},
						"Graphd": map[string]any{
							"Resources": e2ematcher.DeepEqual(
								corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"cpu":    resource.MustParse("500m"),
										"memory": resource.MustParse("500Mi"),
									},
									Limits: corev1.ResourceList{
										"cpu":    resource.MustParse("1"),
										"memory": resource.MustParse("500Mi"),
									},
								},
							),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name: "update graphd resources",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.resources.limits.cpu=1100m",
							"--set", "nebula.graphd.resources.limits.memory=1100Mi",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Resources": e2ematcher.DeepEqual(
										corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												"cpu":    resource.MustParse("500m"),
												"memory": resource.MustParse("500Mi"),
											},
											Limits: corev1.ResourceList{
												"cpu":    resource.MustParse("1100m"),
												"memory": resource.MustParse("1100Mi"),
											},
										},
									),
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name: "update metad resources",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.metad.resources.limits.cpu=1100m",
							"--set", "nebula.metad.resources.limits.memory=1100Mi",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Metad": map[string]any{
									"Resources": e2ematcher.DeepEqual(
										corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												"cpu":    resource.MustParse("500m"),
												"memory": resource.MustParse("500Mi"),
											},
											Limits: corev1.ResourceList{
												"cpu":    resource.MustParse("1100m"),
												"memory": resource.MustParse("1100Mi"),
											},
										},
									),
								},
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
			{
				Name: "update storaged resources",
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.storaged.resources.limits.cpu=1100m",
							"--set", "nebula.storaged.resources.limits.memory=1100Mi",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Storaged": map[string]any{
									"Resources": e2ematcher.DeepEqual(
										corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												"cpu":    resource.MustParse("500m"),
												"memory": resource.MustParse("500Mi"),
											},
											Limits: corev1.ResourceList{
												"cpu":    resource.MustParse("1100m"),
												"memory": resource.MustParse("1100Mi"),
											},
										},
									),
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

// test cases about image of graphd｜metad｜storaged
var testCasesBasicImage = []ncTestCase{
	// TODO
}

// test cases about log and data volume of graphd｜metad｜storaged
var testCasesBasicVolume = []ncTestCase{
	// TODO
}
