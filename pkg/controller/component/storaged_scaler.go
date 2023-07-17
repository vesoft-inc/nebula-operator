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

package component

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nebulago "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
)

type storageScaler struct {
	client.Client
	clientSet kube.ClientSet
}

func NewStorageScaler(cli client.Client, clientSet kube.ClientSet) ScaleManager {
	return &storageScaler{Client: cli, clientSet: clientSet}
}

func (ss *storageScaler) Scale(nc *v1alpha1.NebulaCluster, oldUnstruct, newUnstruct *unstructured.Unstructured) error {
	oldReplicas := extender.GetReplicas(oldUnstruct)
	newReplicas := extender.GetReplicas(newUnstruct)

	if *newReplicas < *oldReplicas || nc.Status.Storaged.Phase == v1alpha1.ScaleInPhase {
		return ss.ScaleIn(nc, *oldReplicas, *newReplicas)
	}

	if *newReplicas > *oldReplicas || nc.Status.Storaged.Phase == v1alpha1.ScaleOutPhase {
		return ss.ScaleOut(nc)
	}

	return nil
}

func (ss *storageScaler) ScaleOut(nc *v1alpha1.NebulaCluster) error {
	ns := nc.GetNamespace()
	componentName := nc.StoragedComponent().GetName()
	nc.Status.Storaged.Phase = v1alpha1.ScaleOutPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	if !nc.StoragedComponent().IsReady() {
		klog.Infof("cluster [%s/%s] status not ready", ns, componentName)
		return nil
	}

	if !pointer.BoolDeref(nc.Spec.Storaged.EnableAutoBalance, false) {
		klog.Infof("cluster [%s/%s] auto balance is disabled", ns, componentName)
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
		return nil
	}

	options, err := nebula.ClientOptions(nc)
	if err != nil {
		return err
	}
	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints, options...)
	if err != nil {
		klog.Errorf("create meta client failed: %v", err)
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			klog.Errorf("disconnect meta client failed: %v", err)
		}
	}()

	spaces, err := metaClient.ListSpaces()
	if err != nil {
		return err
	}

	if len(spaces) > 0 && nc.Status.Storaged.BalancedSpaces == nil {
		nc.Status.Storaged.BalancedSpaces = make([]int32, 0, len(spaces))
	}

	for _, space := range spaces {
		if contains(nc.Status.Storaged.BalancedSpaces, *space.Id.SpaceID) {
			continue
		}
		if err := ss.balanceSpace(metaClient, nc, *space.Id.SpaceID); err != nil {
			return err
		}
	}

	nc.Status.Storaged.BalancedSpaces = nil
	nc.Status.Storaged.LastBalanceJob = nil
	nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	return nil
}

// nolint: revive
func (ss *storageScaler) ScaleIn(nc *v1alpha1.NebulaCluster, oldReplicas, newReplicas int32) error {
	ns := nc.GetNamespace()
	ncName := nc.GetName()
	componentName := nc.StoragedComponent().GetName()
	nc.Status.Storaged.Phase = v1alpha1.ScaleInPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	options, err := nebula.ClientOptions(nc)
	if err != nil {
		return err
	}
	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints, options...)
	if err != nil {
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			klog.Error("meta client disconnect failed: %v", err)
		}
	}()

	spaces, err := metaClient.ListSpaces()
	if err != nil {
		return err
	}

	if oldReplicas-newReplicas > 0 {
		scaleSets := sets.NewString()
		hosts := make([]*nebulago.HostAddr, 0, oldReplicas-newReplicas)
		port := nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift)
		for i := oldReplicas - 1; i >= newReplicas; i-- {
			host := nc.StoragedComponent().GetPodFQDN(i)
			hosts = append(hosts, &nebulago.HostAddr{
				Host: host,
				Port: port,
			})
			scaleSets.Insert(host)
		}
		if len(spaces) > 0 {
			for _, space := range spaces {
				leaderSets, err := metaClient.GetSpaceLeaderHosts(space.Name)
				if err != nil {
					return err
				}
				removed := filterRemovedHosts(sets.NewString(leaderSets...), scaleSets, hosts)
				if len(removed) == 0 {
					continue
				}
				if err := ss.removeHost(metaClient, nc, *space.Id.SpaceID, hosts); err != nil {
					return err
				}
				klog.Infof("cluster [%s/%s] remove hosts from space %s successfully", ns, ncName, space.Name)
			}
		}
		if err := metaClient.DropHosts(hosts); err != nil {
			return err
		}
		klog.Infof("cluster [%s/%s] drop hosts successfully", ns, ncName)
	}

	if len(spaces) > 0 && nc.Status.Storaged.BalancedSpaces == nil {
		nc.Status.Storaged.BalancedSpaces = make([]int32, 0, len(spaces))
	}

	for _, space := range spaces {
		if contains(nc.Status.Storaged.BalancedSpaces, *space.Id.SpaceID) {
			continue
		}
		if err := metaClient.BalanceLeader(*space.Id.SpaceID); err != nil {
			return err
		}
		if nc.Status.Storaged.LastBalanceJob != nil {
			nc.Status.Storaged.BalancedSpaces = append(nc.Status.Storaged.BalancedSpaces, nc.Status.Storaged.LastBalanceJob.SpaceID)
		}
	}

	if err := PVCMark(ss.clientSet.PVC(), nc.StoragedComponent(), oldReplicas, newReplicas); err != nil {
		return err
	}

	deleted := true
	pvcNames := ordinalPVCNames(nc.StoragedComponent().ComponentType(), nc.StoragedComponent().GetName(), newReplicas)
	for _, pvcName := range pvcNames {
		if _, err = ss.clientSet.PVC().GetPVC(nc.GetNamespace(), pvcName); err != nil {
			if !apierrors.IsNotFound(err) {
				deleted = false
				break
			}
		}
	}
	if !deleted {
		return &utilerrors.ReconcileError{Msg: fmt.Sprintf("pvc reclaim %s still in progress",
			nc.StoragedComponent().GetName())}
	}

	if nc.StoragedComponent().IsReady() {
		klog.Infof("cluster [%s/%s] all used pvcs were reclaimed", ns, componentName)
		nc.Status.Storaged.BalancedSpaces = nil
		nc.Status.Storaged.LastBalanceJob = nil
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	}
	return nil
}

func (ss *storageScaler) balanceSpace(mc nebula.MetaInterface, nc *v1alpha1.NebulaCluster, spaceID nebulago.GraphSpaceID) error {
	if nc.Status.Storaged.LastBalanceJob != nil && nc.Status.Storaged.LastBalanceJob.SpaceID == spaceID {
		if err := mc.BalanceStatus(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID); err != nil {
			return err
		}
		if err := mc.BalanceLeader(nc.Status.Storaged.LastBalanceJob.SpaceID); err != nil {
			return err
		}
		nc.Status.Storaged.BalancedSpaces = append(nc.Status.Storaged.BalancedSpaces, nc.Status.Storaged.LastBalanceJob.SpaceID)
		return nil
	}
	jobID, err := mc.BalanceData(spaceID)
	if err != nil {
		if jobID > 0 {
			nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
				SpaceID: spaceID,
				JobID:   jobID,
			}
		}
		return err
	}
	return nil
}

func (ss *storageScaler) removeHost(
	mc nebula.MetaInterface,
	nc *v1alpha1.NebulaCluster,
	spaceID nebulago.GraphSpaceID,
	hosts []*nebulago.HostAddr,
) error {
	if nc.Status.Storaged.LastBalanceJob != nil && nc.Status.Storaged.LastBalanceJob.SpaceID == spaceID {
		return mc.BalanceStatus(nc.Status.Storaged.LastBalanceJob.JobID, nc.Status.Storaged.LastBalanceJob.SpaceID)
	}
	jobID, err := mc.RemoveHost(spaceID, hosts)
	if err != nil {
		if jobID > 0 {
			nc.Status.Storaged.LastBalanceJob = &v1alpha1.BalanceJob{
				SpaceID: spaceID,
				JobID:   jobID,
			}
		}
		return err
	}
	return nil
}

func filterRemovedHosts(leaderSets, scaleSets sets.String, scaledHosts []*nebulago.HostAddr) []*nebulago.HostAddr {
	result := sets.NewString()
	for key := range scaleSets {
		if leaderSets.Has(key) {
			result.Insert(key)
		}
	}
	if len(result) == 0 {
		return nil
	}
	removed := make([]*nebulago.HostAddr, 0)
	for _, host := range scaledHosts {
		if result.Has(host.Host) {
			removed = append(removed, host)
		}
	}
	return removed
}
