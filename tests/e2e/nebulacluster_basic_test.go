package e2e

import (
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2evalidator"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/e2e-framework/third_party/helm"
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
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]e2evalidator.Rule{
					"Spec.Graphd.Replicas":   e2evalidator.Eq(2),
					"Spec.Metad.Replicas":    e2evalidator.Eq(3),
					"Spec.Storaged.Replicas": e2evalidator.Eq(3),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(4),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(4),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(5),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(4),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(5),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(5),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(3),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(4),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(3),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(3),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":   e2evalidator.Eq(2),
							"Spec.Metad.Replicas":    e2evalidator.Eq(3),
							"Spec.Storaged.Replicas": e2evalidator.Eq(3),
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
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]e2evalidator.Rule{
					"Spec.Graphd.Replicas":            e2evalidator.Eq(2),
					"Spec.Metad.Replicas":             e2evalidator.Eq(3),
					"Spec.Storaged.Replicas":          e2evalidator.Eq(3),
					"Spec.Graphd.Version":             e2evalidator.Ne("latest"),
					"Spec.Metad.Version":              e2evalidator.Ne("latest"),
					"Spec.Storaged.Version":           e2evalidator.Ne("latest"),
					"Spec.Storaged.EnableForceUpdate": e2evalidator.Eq(false),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":            e2evalidator.Eq(2),
							"Spec.Metad.Replicas":             e2evalidator.Eq(3),
							"Spec.Storaged.Replicas":          e2evalidator.Eq(3),
							"Spec.Graphd.Version":             e2evalidator.Eq("latest"),
							"Spec.Metad.Version":              e2evalidator.Eq("latest"),
							"Spec.Storaged.Version":           e2evalidator.Eq("latest"),
							"Spec.Storaged.EnableForceUpdate": e2evalidator.Eq(false),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":            e2evalidator.Eq(4),
							"Spec.Metad.Replicas":             e2evalidator.Eq(3),
							"Spec.Storaged.Replicas":          e2evalidator.Eq(4),
							"Spec.Graphd.Version":             e2evalidator.Ne("latest"),
							"Spec.Metad.Version":              e2evalidator.Ne("latest"),
							"Spec.Storaged.Version":           e2evalidator.Ne("latest"),
							"Spec.Storaged.EnableForceUpdate": e2evalidator.Eq(false),
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
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]e2evalidator.Rule{
							"Spec.Graphd.Replicas":            e2evalidator.Eq(2),
							"Spec.Metad.Replicas":             e2evalidator.Eq(3),
							"Spec.Storaged.Replicas":          e2evalidator.Eq(3),
							"Spec.Graphd.Version":             e2evalidator.Eq("latest"),
							"Spec.Metad.Version":              e2evalidator.Eq("latest"),
							"Spec.Storaged.Version":           e2evalidator.Eq("latest"),
							"Spec.Storaged.EnableForceUpdate": e2evalidator.Eq(true),
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
				envfuncsext.NebulaClusterReadyFuncForMetadResource(corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}),
				envfuncsext.NebulaClusterReadyFuncForStoragedResource(corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}),
				envfuncsext.NebulaClusterReadyFuncForGraphdResource(corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
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
						envfuncsext.NebulaClusterReadyFuncForGraphdResource(corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("1100m"),
								corev1.ResourceMemory: resource.MustParse("1100Mi"),
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
						envfuncsext.NebulaClusterReadyFuncForMetadResource(corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("1100m"),
								corev1.ResourceMemory: resource.MustParse("1100Mi"),
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
						envfuncsext.NebulaClusterReadyFuncForStoragedResource(corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("1100m"),
								corev1.ResourceMemory: resource.MustParse("1100Mi"),
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
