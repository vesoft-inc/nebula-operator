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
	corev1 "k8s.io/api/core/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
)

type ClusterConditionUpdater interface {
	Update(cluster *v1alpha1.NebulaCluster)
}

var _ ClusterConditionUpdater = &nebulaClusterConditionUpdater{}

func NewClusterConditionUpdater() ClusterConditionUpdater {
	return &nebulaClusterConditionUpdater{}
}

type nebulaClusterConditionUpdater struct{}

func (u *nebulaClusterConditionUpdater) Update(nc *v1alpha1.NebulaCluster) {
	u.updateReadyCondition(nc)
}

func allWorkloadsAreUpToDate(nc *v1alpha1.NebulaCluster) bool {
	isUpToDate := func(status *v1alpha1.WorkloadStatus, requireExist bool) bool {
		if status == nil {
			return !requireExist
		}
		return status.CurrentRevision == status.UpdateRevision
	}

	updated := (isUpToDate(nc.Status.Metad.Workload, false)) &&
		(isUpToDate(nc.Status.Storaged.Workload, false)) &&
		(isUpToDate(nc.Status.Graphd.Workload, false))

	return updated
}

func (u *nebulaClusterConditionUpdater) updateReadyCondition(nc *v1alpha1.NebulaCluster) {
	status := corev1.ConditionFalse
	var reason string
	var message string

	switch {
	case !allWorkloadsAreUpToDate(nc):
		reason = condition.WorkloadNotUpToDate
		message = "Workload is in progress"
	case !nc.GraphdComponent().IsReady():
		reason = condition.GraphdUnhealthy
		message = "Graphd is not healthy"
	case !nc.MetadComponent().IsReady():
		reason = condition.MetadUnhealthy
		message = "Metad is not healthy"
	case !nc.IsStoragedAvailable():
		reason = condition.StoragedUnhealthy
		message = "Storaged is not healthy"
	default:
		status = corev1.ConditionTrue
		reason = condition.WorkloadReady
		message = "Nebula cluster is running"
	}

	cond := condition.NewNebulaClusterCondition(v1alpha1.NebulaClusterReady, status, reason, message)
	condition.SetNebulaClusterCondition(&nc.Status, cond)
}
