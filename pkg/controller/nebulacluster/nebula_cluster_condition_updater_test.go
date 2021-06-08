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

package nebulacluster

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

func Test_nebulaClusterConditionUpdater_updateReadyCondition(t *testing.T) {
	tests := []struct {
		name        string
		nc          *v1alpha1.NebulaCluster
		wantStatus  corev1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name: "workload not up to date",
			nc: &v1alpha1.NebulaCluster{
				Spec: v1alpha1.NebulaClusterSpec{
					Graphd:   &v1alpha1.GraphdSpec{},
					Metad:    &v1alpha1.MetadSpec{},
					Storaged: &v1alpha1.StoragedSpec{},
				},
				Status: v1alpha1.NebulaClusterStatus{
					Graphd: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "11",
							UpdateRevision:  "12",
						},
					},
					Metad: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "11",
							UpdateRevision:  "12",
						},
					},
					Storaged: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "11",
							UpdateRevision:  "12",
						},
					},
				},
			},
			wantStatus:  corev1.ConditionFalse,
			wantReason:  condition.WorkloadNotUpToDate,
			wantMessage: "Workload is in progress",
		},
		{
			name: "graphd not ready",
			nc: &v1alpha1.NebulaCluster{
				Spec: v1alpha1.NebulaClusterSpec{
					Graphd: &v1alpha1.GraphdSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Metad:    &v1alpha1.MetadSpec{},
					Storaged: &v1alpha1.StoragedSpec{},
				},
				Status: v1alpha1.NebulaClusterStatus{
					Graphd: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   2,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Metad: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Storaged: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
				},
			},
			wantStatus:  corev1.ConditionFalse,
			wantReason:  condition.GraphdUnhealthy,
			wantMessage: "Graphd is not healthy",
		},
		{
			name: "metad not ready",
			nc: &v1alpha1.NebulaCluster{
				Spec: v1alpha1.NebulaClusterSpec{
					Graphd: &v1alpha1.GraphdSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Metad: &v1alpha1.MetadSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Storaged: &v1alpha1.StoragedSpec{},
				},
				Status: v1alpha1.NebulaClusterStatus{
					Graphd: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Metad: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   2,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Storaged: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
				},
			},
			wantStatus:  corev1.ConditionFalse,
			wantReason:  condition.MetadUnhealthy,
			wantMessage: "Metad is not healthy",
		},
		{
			name: "storaged not ready",
			nc: &v1alpha1.NebulaCluster{
				Spec: v1alpha1.NebulaClusterSpec{
					Graphd: &v1alpha1.GraphdSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Metad: &v1alpha1.MetadSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Storaged: &v1alpha1.StoragedSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
				},
				Status: v1alpha1.NebulaClusterStatus{
					Graphd: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Metad: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Storaged: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   2,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
				},
			},
			wantStatus:  corev1.ConditionFalse,
			wantReason:  condition.StoragedUnhealthy,
			wantMessage: "Storaged is not healthy",
		},
		{
			name: "all are ready",
			nc: &v1alpha1.NebulaCluster{
				Spec: v1alpha1.NebulaClusterSpec{
					Graphd: &v1alpha1.GraphdSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Metad: &v1alpha1.MetadSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
					Storaged: &v1alpha1.StoragedSpec{
						PodSpec: v1alpha1.PodSpec{
							Replicas: pointer.Int32Ptr(3),
						},
					},
				},
				Status: v1alpha1.NebulaClusterStatus{
					Graphd: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Metad: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
					Storaged: v1alpha1.ComponentStatus{
						Workload: v1alpha1.WorkloadStatus{
							Replicas:        3,
							ReadyReplicas:   3,
							CurrentRevision: "12",
							UpdateRevision:  "12",
						},
					},
				},
			},
			wantStatus:  corev1.ConditionTrue,
			wantReason:  condition.WorkloadReady,
			wantMessage: "Nebula cluster is running",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &nebulaClusterConditionUpdater{}
			u.Update(tt.nc)
			cond := condition.GetNebulaClusterCondition(&tt.nc.Status, v1alpha1.NebulaClusterReady)
			if diff := cmp.Diff(tt.wantStatus, cond.Status); diff != "" {
				t.Errorf("unexpected status: %s", diff)
			}
			if diff := cmp.Diff(tt.wantReason, cond.Reason); diff != "" {
				t.Errorf("unexpected reason: %s", diff)
			}
			if diff := cmp.Diff(tt.wantMessage, cond.Message); diff != "" {
				t.Errorf("unexpected message: %s", diff)
			}
		})
	}
}
