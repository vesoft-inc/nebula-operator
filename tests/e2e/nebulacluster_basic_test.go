package e2e

import (
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

const (
	LabelCategoryBasic = "basic"
	LabelGroupScale    = "scale"
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
				envfuncsext.NebulaClusterReadyFuncForReplicas(2, 3, 3),
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

// test cases about version of graphd｜metad｜storaged
var testCasesBasicVersion = []ncTestCase{
	// TODO
}

// test cases about resources of graphd｜metad｜storaged
var testCasesBasicResources = []ncTestCase{
	// TODO
}

// test cases about image of graphd｜metad｜storaged
var testCasesBasicImage = []ncTestCase{
	// TODO
}

// test cases about log and data volume of graphd｜metad｜storaged
var testCasesBasicVolume = []ncTestCase{
	// TODO
}
