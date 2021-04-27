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
	// AnnPVCDeferDeletingKey is pvc defer deletion annotation key used in PVC for defer deleting PVC
	AnnPVCDeferDeletingKey = "nebula-graph.io/pvc-defer-deleting"
	// AnnPodNameKey is pod name annotation key used in PV/PVC for synchronizing nebula cluster meta info
	AnnPodNameKey string = "nebula-graph.io/pod-name"
	// AnnLastSyncTimestampKey is annotation key to indicate the last timestamp the operator sync the workload
	AnnLastSyncTimestampKey = "nebula-graph.io/sync-timestamp"
	// AnnHaModeKey is annotation key to indicate whether in ha mode
	AnnHaModeKey = "nebula-graph.io/ha-mode"
	// AnnLastAppliedConfigKey is annotation key to indicate the last applied configuration
	AnnLastAppliedConfigKey = "nebula-graph.io/last-applied-configuration"
	// AnnPodSchedulingKey is pod scheduling annotation key, it represents whether the pod is scheduling
	AnnPodSchedulingKey = "nebula-graph.io/pod-scheduling"
	// AnnPodConfigMapHash is pod configmap hash key to update configuration dynamically
	AnnPodConfigMapHash = "nebula-graph.io/cm-hash"

	// AnnHaModeVal is annotation value to indicate whether in ha mode
	AnnHaModeVal = "true"
)

// IsInHaMode check whether in ha mode
func IsInHaMode(ann map[string]string) bool {
	if ann != nil {
		val, ok := ann[AnnHaModeKey]
		if ok && val == AnnHaModeVal {
			return true
		}
	}
	return false
}
