package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	LabelZone        = "zone"
	LabelZoneScale   = "scale"
	LabelZoneVersion = "version"
)

var testCasesZone []ncTestCase

func init() {
	testCasesZone = append(testCasesZone, testCasesZoneExporter...)
	testCasesZone = append(testCasesZone, testCasesZoneConsole...)
}

// test cases about zone scale
var testCasesZoneExporter = []ncTestCase{
	{
		Name: "scale with zone",
		Labels: map[string]string{
			LabelKeyCategory: LabelZone,
			LabelKeyGroup:    LabelZoneScale,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
					"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
					"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
					"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
					"--set-string", "nebula.schedulerName=nebula-scheduler",
					"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
					"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
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
								"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
								"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
							},
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"Config": map[string]any{
								"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
							},
						},
						"Storaged": map[string]any{
							"Replicas":          e2ematcher.ValidatorEq(3),
							"EnableAutoBalance": e2ematcher.ValidatorEq(false),
						},
						"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
						"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
						"TopologySpreadConstraints": map[string]any{
							"0": map[string]any{
								"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
								"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
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
				Name:        "scale out [graphd, storaged]: 4-3-4",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.graphd.replicas=4",
							"--set", "nebula.metad.replicas=3",
							"--set", "nebula.storaged.replicas=4",
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(4),
									"EnableAutoBalance": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(5),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(4),
									"EnableAutoBalance": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set", "nebula.storaged.enableAutoBalance=true",
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(5),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(5),
									"EnableAutoBalance": e2ematcher.ValidatorEq(true),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(4),
									"EnableAutoBalance": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set", "nebula.storaged.enableAutoBalance=true",
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"EnableAutoBalance": e2ematcher.ValidatorEq(true),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"EnableAutoBalance": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
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

// test cases about zone version
var testCasesZoneConsole = []ncTestCase{
	{
		Name: "update version with zone",
		Labels: map[string]string{
			LabelKeyCategory: LabelZone,
			LabelKeyGroup:    LabelZoneVersion,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
					"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
					"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
					"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
					"--set-string", "nebula.schedulerName=nebula-scheduler",
					"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
					"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
							"Version":  e2ematcher.ValidatorNe("latest"),
							"Config": map[string]any{
								"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
								"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
							},
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
							"Version":  e2ematcher.ValidatorNe("latest"),
							"Config": map[string]any{
								"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
							},
						},
						"Storaged": map[string]any{
							"Replicas":          e2ematcher.ValidatorEq(3),
							"Version":           e2ematcher.ValidatorNe("latest"),
							"EnableForceUpdate": e2ematcher.ValidatorEq(false),
						},
						"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
						"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
						"TopologySpreadConstraints": map[string]any{
							"0": map[string]any{
								"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
								"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
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
				Name:        "update version",
				UpgradeFunc: nil,
				UpgradeNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterHelmRawOptions(
						helm.WithArgs(
							"--set", "nebula.version=latest",
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterWaitOptions(wait.WithTimeout(time.Minute * 8)),
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Version":  e2ematcher.ValidatorEq("latest"),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorEq("latest"),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"Version":           e2ematcher.ValidatorEq("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterWaitOptions(wait.WithTimeout(time.Minute * 8)),
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(4),
									"Version":  e2ematcher.ValidatorNe("latest"),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorNe("latest"),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(4),
									"Version":           e2ematcher.ValidatorNe("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(false),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
									},
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
							"--set-string", "nebula.alpineImage=vesoft/nebula-alpine:latest",
							"--set-string", fmt.Sprintf("'nebula.metad.config.zone_list=%s'", strings.Join(config.C.NebulaGraph.Zones, "\\,")),
							"--set-string", "nebula.graphd.config.prioritize_intra_zone_reading=1",
							"--set-string", "nebula.graphd.config.stick_to_intra_zone_on_failure=1",
							"--set-string", "nebula.schedulerName=nebula-scheduler",
							"--set-string", "nebula.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone",
							"--set-string", "nebula.topologySpreadConstraints[0].whenUnsatisfiable=DoNotSchedule",
						),
					),
				},
				UpgradeWaitNCOptions: []envfuncsext.NebulaClusterOption{
					envfuncsext.WithNebulaClusterWaitOptions(wait.WithTimeout(time.Minute * 8)),
					envfuncsext.WithNebulaClusterReadyFuncs(
						envfuncsext.NebulaClusterReadyFuncForFields(true, map[string]any{
							"Spec": map[string]any{
								"Graphd": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(2),
									"Version":  e2ematcher.ValidatorEq("latest"),
									"Config": map[string]any{
										"prioritize_intra_zone_reading":  e2ematcher.ValidatorEq("1"),
										"stick_to_intra_zone_on_failure": e2ematcher.ValidatorEq("1"),
									},
								},
								"Metad": map[string]any{
									"Replicas": e2ematcher.ValidatorEq(3),
									"Version":  e2ematcher.ValidatorEq("latest"),
									"Config": map[string]any{
										"zone_list": e2ematcher.ValidatorEq(strings.Join(config.C.NebulaGraph.Zones, ",")),
									},
								},
								"Storaged": map[string]any{
									"Replicas":          e2ematcher.ValidatorEq(3),
									"Version":           e2ematcher.ValidatorEq("latest"),
									"EnableForceUpdate": e2ematcher.ValidatorEq(true),
								},
								"AlpineImage":   e2ematcher.ValidatorEq("vesoft/nebula-alpine:latest"),
								"SchedulerName": e2ematcher.ValidatorEq("nebula-scheduler"),
								"TopologySpreadConstraints": map[string]any{
									"0": map[string]any{
										"TopologyKey":       e2ematcher.ValidatorEq("topology.kubernetes.io/zone"),
										"WhenUnsatisfiable": e2ematcher.ValidatorEq(corev1.DoNotSchedule),
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
