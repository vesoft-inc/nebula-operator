package e2e

import (
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelTools         = "tools"
	LabelToolsExporter = "exporter"
	LabelToolsConsole  = "console"
)

var testCasesTools []ncTestCase

func init() {
	testCasesTools = append(testCasesTools, testCasesToolsExporter...)
	testCasesTools = append(testCasesTools, testCasesToolsConsole...)
}

// test cases about tools exporter
var testCasesToolsExporter = []ncTestCase{
	{
		Name: "tools for exporter",
		Labels: map[string]string{
			LabelKeyCategory: LabelTools,
			LabelKeyGroup:    LabelToolsExporter,
		},
		DefaultNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set", "nebula.graphd.replicas=1",
					"--set", "nebula.metad.replicas=1",
					"--set", "nebula.storaged.replicas=1",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Exporter": map[string]any{
							"Version": e2ematcher.ValidatorNe("latest"),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC: false,
		UpgradeCases: []ncTestUpgradeCase{
			{
				Name:        "update version",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set-string", "nebula.exporter.version=latest",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Exporter": map[string]any{
									"Version": e2ematcher.ValidatorEq("latest"),
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

// test cases about tools console
var testCasesToolsConsole = []ncTestCase{
	// TODO
}
