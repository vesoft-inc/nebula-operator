package e2e

import (
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelPV          = "pv"
	LabelPVExpansion = "expansion"
)

var testCasesPV []ncTestCase

func init() {
	testCasesPV = append(testCasesPV, testCasesPVExpansion...)
}

// test cases about pv expansion
var testCasesPVExpansion = []ncTestCase{
	{
		Name: "pv expansion",
		Labels: map[string]string{
			LabelKeyCategory: LabelPV,
			LabelKeyGroup:    LabelPVExpansion,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set", "nebula.graphd.logStorage=1Gi",
					"--set", "nebula.metad.logStorage=2Gi",
					"--set", "nebula.metad.dataStorage=3Gi",
					"--set", "nebula.storaged.logStorage=4Gi",
					"--set", "nebula.storaged.dataStorage=5Gi",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("1Gi"),
										},
									},
								),
							},
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("2Gi"),
										},
									},
								),
							},
							"DataVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("3Gi"),
										},
									},
								),
							},
						},
						"Storaged": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("4Gi"),
										},
									},
								),
							},
							"DataVolumeClaims": map[string]any{
								"0": map[string]any{
									"Resources": e2ematcher.DeepEqual(
										corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("5Gi"),
											},
										},
									),
								},
							},
						},
						"EnablePVReclaim": e2ematcher.ValidatorEq(false),
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update storage size",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.logStorage=2Gi",
							"--set", "nebula.metad.logStorage=3Gi",
							"--set", "nebula.metad.dataStorage=4Gi",
							"--set", "nebula.storaged.logStorage=5Gi",
							"--set", "nebula.storaged.dataStorage=6Gi",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("2Gi"),
												},
											},
										),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("3Gi"),
												},
											},
										),
									},
									"DataVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("4Gi"),
												},
											},
										),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("5Gi"),
												},
											},
										),
									},
									"DataVolumeClaims": map[string]any{
										"0": map[string]any{
											"Resources": e2ematcher.DeepEqual(
												corev1.ResourceRequirements{
													Requests: corev1.ResourceList{
														"storage": resource.MustParse("6Gi"),
													},
												},
											),
										},
									},
								},
								"EnablePVReclaim": e2ematcher.ValidatorEq(false),
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
		},
	},
	{
		Name: "pv expansion with EnablePVReclaim",
		Labels: map[string]string{
			LabelKeyCategory: LabelPV,
			LabelKeyGroup:    LabelPVExpansion,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set", "nebula.enablePVReclaim=true",
					"--set", "nebula.graphd.logStorage=1Gi",
					"--set", "nebula.metad.logStorage=2Gi",
					"--set", "nebula.metad.dataStorage=3Gi",
					"--set", "nebula.storaged.logStorage=4Gi",
					"--set", "nebula.storaged.dataStorage=5Gi",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("1Gi"),
										},
									},
								),
							},
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("2Gi"),
										},
									},
								),
							},
							"DataVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("3Gi"),
										},
									},
								),
							},
						},
						"Storaged": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"LogVolumeClaim": map[string]any{
								"Resources": e2ematcher.DeepEqual(
									corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("4Gi"),
										},
									},
								),
							},
							"DataVolumeClaims": map[string]any{
								"0": map[string]any{
									"Resources": e2ematcher.DeepEqual(
										corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("5Gi"),
											},
										},
									),
								},
							},
						},
						"EnablePVReclaim": e2ematcher.ValidatorEq(true),
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: true,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update storage size",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.enablePVReclaim=true",
							"--set", "nebula.graphd.logStorage=2Gi",
							"--set", "nebula.metad.logStorage=3Gi",
							"--set", "nebula.metad.dataStorage=4Gi",
							"--set", "nebula.storaged.logStorage=5Gi",
							"--set", "nebula.storaged.dataStorage=6Gi",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("2Gi"),
												},
											},
										),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("3Gi"),
												},
											},
										),
									},
									"DataVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("4Gi"),
												},
											},
										),
									},
								},
								"Storaged": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"LogVolumeClaim": map[string]any{
										"Resources": e2ematcher.DeepEqual(
											corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													"storage": resource.MustParse("5Gi"),
												},
											},
										),
									},
									"DataVolumeClaims": map[string]any{
										"0": map[string]any{
											"Resources": e2ematcher.DeepEqual(
												corev1.ResourceRequirements{
													Requests: corev1.ResourceList{
														"storage": resource.MustParse("6Gi"),
													},
												},
											),
										},
									},
								},
								"EnablePVReclaim": e2ematcher.ValidatorEq(true),
							},
						}),
						envfuncsext.DefaultNebulaClusterReadyFunc,
					),
				},
			},
		},
	},
}
