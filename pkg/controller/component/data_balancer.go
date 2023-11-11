/*
Copyright 2023 Vesoft Inc.

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

package component

import (
	nebulago "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"k8s.io/klog/v2"
)

func balanceSpace(clientSet kube.ClientSet, mc nebula.MetaInterface, nc *v1alpha1.NebulaCluster, spaceID nebulago.GraphSpaceID) error {
	if nc.Status.Storaged.LastBalanceJob != nil && nc.Status.Storaged.LastBalanceJob.SpaceID == spaceID {
		if err := mc.BalanceStatus(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID); err != nil {
			if err == nebula.ErrJobStatusFailed {
				klog.Infof("space %d balance job %d will be recovered", nc.Status.Storaged.LastBalanceJob.SpaceID, nc.Status.Storaged.LastBalanceJob.JobID)
				if nc.IsZoneEnabled() {
					return mc.RecoverInZoneBalanceJob(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID)
				}
				return mc.RecoverDataBalanceJob(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID)
			}
			return err
		}
		if err := mc.BalanceLeader(nc.Status.Storaged.LastBalanceJob.SpaceID); err != nil {
			return err
		}
		nc.Status.Storaged.BalancedSpaces = append(nc.Status.Storaged.BalancedSpaces, nc.Status.Storaged.LastBalanceJob.SpaceID)
		return clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc)
	}

	namespace := nc.GetNamespace()
	componentName := nc.StoragedComponent().GetName()
	if nc.IsZoneEnabled() {
		jobID, err := mc.BalanceDataInZone(spaceID)
		if err != nil {
			klog.Errorf("storaged cluster [%s/%s] balance data in zone error: %v", namespace, componentName, err)
			if jobID > 0 {
				nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
					SpaceID: spaceID,
					JobID:   jobID,
				}
				if err := clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc); err != nil {
					return err
				}
			}
			return err
		}
		return nil
	}

	jobID, err := mc.BalanceData(spaceID)
	if err != nil {
		klog.Errorf("storaged cluster [%s/%s] balance data across zone error: %v", namespace, componentName, err)
		if jobID > 0 {
			nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
				SpaceID: spaceID,
				JobID:   jobID,
			}
			if err := clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}

func removeHost(
	clientSet kube.ClientSet,
	mc nebula.MetaInterface,
	nc *v1alpha1.NebulaCluster,
	spaceID nebulago.GraphSpaceID,
	hosts []*nebulago.HostAddr,
) error {
	if nc.Status.Storaged.LastBalanceJob != nil && nc.Status.Storaged.LastBalanceJob.SpaceID == spaceID {
		if err := mc.BalanceStatus(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID); err != nil {
			if err == nebula.ErrJobStatusFailed {
				klog.Infof("space %d balance job %d will be recovered", nc.Status.Storaged.LastBalanceJob.SpaceID, nc.Status.Storaged.LastBalanceJob.JobID)
				if nc.IsZoneEnabled() {
					return mc.RecoverInZoneBalanceJob(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID)
				}
				return mc.RecoverDataBalanceJob(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID)
			}
			return err
		}
		nc.Status.Storaged.RemovedSpaces = append(nc.Status.Storaged.RemovedSpaces, nc.Status.Storaged.LastBalanceJob.SpaceID)
		return clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc)
	}

	namespace := nc.GetNamespace()
	componentName := nc.StoragedComponent().GetName()
	if nc.IsZoneEnabled() {
		jobID, err := mc.RemoveHostInZone(spaceID, hosts)
		klog.Errorf("storaged cluster [%s/%s] remove host in zone error: %v", namespace, componentName, err)
		if err != nil {
			if jobID > 0 {
				nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
					SpaceID: spaceID,
					JobID:   jobID,
				}
				if err := clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc); err != nil {
					return err
				}
			}
			return err
		}
		return nil
	}

	jobID, err := mc.RemoveHost(spaceID, hosts)
	if err != nil {
		klog.Errorf("storaged cluster [%s/%s] remove host across zone error: %v", namespace, componentName, err)
		if jobID > 0 {
			nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
				SpaceID: spaceID,
				JobID:   jobID,
			}
			if err := clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}
