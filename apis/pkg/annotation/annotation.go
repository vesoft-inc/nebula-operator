/*
Copyright 2021 Vesoft Inc.

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

package annotation

const (
	AnnDeploymentRevision = "deployment.kubernetes.io/revision"
	// AnnPVCDeferDeletingKey is pvc defer deletion annotation key used in PVC for defer deleting PVC
	AnnPVCDeferDeletingKey = "nebula-graph.io/pvc-defer-deleting"
	// AnnPodNameKey is pod name annotation key used in PV/PVC for synchronizing nebula cluster meta info
	AnnPodNameKey = "nebula-graph.io/pod-name"
	// AnnLastSyncTimestampKey is annotation key to indicate the last timestamp the operator sync the workload
	AnnLastSyncTimestampKey = "nebula-graph.io/sync-timestamp"
	// AnnHaModeKey is annotation key to indicate whether in HA mode
	AnnHaModeKey = "nebula-graph.io/ha-mode"
	// AnnLastReplicas is annotation key to indicate the last replicas
	AnnLastReplicas = "nebula-graph.io/last-replicas"
	// AnnLastAppliedDynamicFlagsKey is annotation key to indicate the last applied custom dynamic flags
	AnnLastAppliedDynamicFlagsKey = "nebula-graph.io/last-applied-dynamic-flags"
	// AnnLastAppliedStaticFlagsKey is annotation key to indicate the last applied custom static flags
	AnnLastAppliedStaticFlagsKey = "nebula-graph.io/last-applied-static-flags"
	// AnnLastAppliedConfigKey is annotation key to indicate the last applied configuration
	AnnLastAppliedConfigKey = "nebula-graph.io/last-applied-configuration"
	// AnnPodConfigMapHash is pod configmap hash key to update configuration dynamically
	AnnPodConfigMapHash = "nebula-graph.io/cm-hash"
	// AnnPvReclaimKey is annotation key that indicate whether reclaim persistent volume
	AnnPvReclaimKey = "nebula-graph.io/enable-pv-reclaim"

	// AnnRestartTimestamp is annotation key to indicate the timestamp that operator restart the workload
	AnnRestartTimestamp = "nebula-graph.io/restart-timestamp"
	// AnnRestartPodOrdinal is the annotation key to indicate which Pod will be restarted
	AnnRestartPodOrdinal = "nebula-graph.io/restart-ordinal"

	// AnnRestoreNameKey is restore name annotation key used for creating new nebula cluster with backup data
	AnnRestoreNameKey = "nebula-graph.io/restore-name"
	// AnnRestoreMetadStepKey is the annotation key to control Metad reconcile process
	AnnRestoreMetadStepKey = "nebula-graph.io/restore-metad-done"
	// AnnRestoreStoragedStepKey is the annotation key to control Storaged reconcile process
	AnnRestoreStoragedStepKey = "nebula-graph.io/restore-storaged-done"
	// AnnRestoreStageKey is the annotation key to indicate what is the current stage
	AnnRestoreStageKey = "restore-stage"

	// AnnHaModeVal is annotation value to indicate whether in HA mode
	AnnHaModeVal = "true"

	// AnnRestoreMetadStepVal is annotation value to indicate whether Metad restore step is completed in stage 1
	AnnRestoreMetadStepVal = "true"
	// AnnRestoreStoragedStepVal is annotation value to indicate whether Storaged restore step is completed in stage 1
	AnnRestoreStoragedStepVal = "true"

	AnnRestoreStage1Val = "restore-stage-1"
	AnnRestoreStage2Val = "restore-stage-2"
)

func IsRestoreNameNotEmpty(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnRestoreNameKey]
		if ok && val != "" {
			return true
		}
	}
	return false
}

func IsRestoreMetadDone(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnRestoreMetadStepKey]
		if ok && val == AnnRestoreMetadStepVal {
			return true
		}
	}
	return false
}

func IsRestoreStoragedDone(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnRestoreStoragedStepKey]
		if ok && val == AnnRestoreStoragedStepVal {
			return true
		}
	}
	return false
}

func IsInRestoreStage2(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnRestoreStageKey]
		if ok && val == AnnRestoreStage2Val {
			return true
		}
	}
	return false
}

// IsInHaMode check whether in HA mode
func IsInHaMode(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnHaModeKey]
		if ok && val == AnnHaModeVal {
			return true
		}
	}
	return false
}

func CopyAnnotations(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := map[string]string{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
