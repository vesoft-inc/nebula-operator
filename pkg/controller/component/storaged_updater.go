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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ng "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

const (
	// TransLeaderBeginTime is the key of trans Leader begin time
	TransLeaderBeginTime = "transLeaderBeginTime"
	// TransLeaderTimeout is the timeout limit of trans leader
	TransLeaderTimeout = 5 * time.Minute
)

type storagedUpdater struct {
	client.Client
	clientSet kube.ClientSet
}

func NewStoragedUpdater(cli client.Client, clientSet kube.ClientSet) UpdateManager {
	return &storagedUpdater{Client: cli, clientSet: clientSet}
}

func (s *storagedUpdater) Update(
	nc *v1alpha1.NebulaCluster,
	oldUnstruct, newUnstruct *unstructured.Unstructured,
	gvk schema.GroupVersionKind,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if *nc.Spec.Storaged.Replicas == int32(0) {
		return nil
	}

	if nc.Status.Storaged.Phase == v1alpha1.ScaleInPhase ||
		nc.Status.Storaged.Phase == v1alpha1.ScaleOutPhase ||
		nc.Status.Metad.Phase == v1alpha1.UpdatePhase {
		return setLastConfig(oldUnstruct, newUnstruct)
	}

	if !extender.PodTemplateEqual(newUnstruct, oldUnstruct) {
		return nil
	}

	if nc.Status.Storaged.Workload.UpdateRevision == nc.Status.Storaged.Workload.CurrentRevision {
		return nil
	}

	spec := extender.GetSpec(oldUnstruct)
	oldStrategy := spec["updateStrategy"].(map[string]interface{})
	advanced := gvk.Kind == resource.AdvancedStatefulSetKind.Kind
	partition := oldStrategy["rollingUpdate"].(map[string]interface{})
	if err := setPartition(newUnstruct, partition["partition"].(int64), advanced); err != nil {
		return err
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	mc, err := nebula.NewMetaClient(endpoints)
	if err != nil {
		return err
	}
	defer func() {
		if err := mc.Disconnect(); err != nil {
			log.Error(err, "meta client disconnect failed")
		}
	}()

	spaces, err := mc.ListSpaces()
	if err != nil {
		return err
	}
	empty := len(spaces) == 0

	replicas := extender.GetReplicas(oldUnstruct)
	index, err := getNextUpdatePod(nc.StoragedComponent(), *replicas, s.clientSet.Pod())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return utilerrors.ReconcileErrorf("%v", err)
		}
		return err
	}
	if index >= 0 {
		return s.updateStoragedPod(mc, nc, index, newUnstruct, advanced, empty)
	}

	return s.updateRunningPhase(mc, nc, spaces)
}

// nolint: revive
func (s *storagedUpdater) updateStoragedPod(
	mc nebula.MetaInterface,
	nc *v1alpha1.NebulaCluster,
	ordinal int32,
	newUnstruct *unstructured.Unstructured,
	advanced bool,
	empty bool,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	namespace := nc.GetNamespace()
	updatePodName := nc.StoragedComponent().GetPodName(ordinal)
	updatePod, err := s.clientSet.Pod().GetPod(namespace, updatePodName)
	if err != nil {
		log.Error(err, "failed to get pod")
		return err
	}

	if empty || *nc.Spec.Storaged.Replicas < 3 {
		return setPartition(newUnstruct, int64(ordinal), advanced)
	}

	_, ok := updatePod.Annotations[TransLeaderBeginTime]
	if !ok {
		return s.transLeaderIfNecessary(nc, mc, ordinal, updatePod)
	}

	podFQDN := nc.StoragedComponent().GetPodFQDN(ordinal)
	if s.readyToUpdate(mc, podFQDN, updatePod) {
		return setPartition(newUnstruct, int64(ordinal), advanced)
	}

	return fmt.Errorf("storaged pod: %s is transferring leader", updatePodName)
}

func (s *storagedUpdater) readyToUpdate(mc nebula.MetaInterface, leaderHost string, updatePod *corev1.Pod) bool {
	log := getLog().WithValues("updatePod", updatePod.GetName())
	count, err := mc.GetLeaderCount(leaderHost)
	if err != nil {
		log.Error(err, "get leader count failed")
		return false
	}
	if count == 0 {
		log.Info("leader count is 0, ready for updating", "host", leaderHost)
		return true
	}
	if timeStr, ok := updatePod.Annotations[TransLeaderBeginTime]; ok {
		transLeaderBeginTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			log.Error(err, "parse time formatted string failed")
			return false
		}
		if time.Now().After(transLeaderBeginTime.Add(TransLeaderTimeout)) {
			log.Error(err, "trans leader timeout")
			return true
		}
	}
	return false
}

func (s *storagedUpdater) transLeaderIfNecessary(
	nc *v1alpha1.NebulaCluster,
	mc nebula.MetaInterface,
	ordinal int32,
	updatePod *corev1.Pod,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if updatePod.Annotations == nil {
		updatePod.Annotations = map[string]string{}
	}
	now := time.Now().Format(time.RFC3339)
	updatePod.Annotations[TransLeaderBeginTime] = now

	if err := s.clientSet.Pod().UpdatePod(updatePod); err != nil {
		return err
	}

	podFQDN := nc.StoragedComponent().GetPodFQDN(ordinal)
	endpoint := fmt.Sprintf("%s:%d", podFQDN, nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameAdmin))
	sc, err := nebula.NewStorageClient([]string{endpoint})
	if err != nil {
		return err
	}

	defer func() {
		if err := sc.Disconnect(); err != nil {
			log.Error(err, "storage client disconnect failed")
		}
	}()

	spaceItems, err := mc.GetSpaceParts()
	if err != nil {
		log.Error(err, "get space partition failed")
		return err
	}
	for spaceID, partItems := range spaceItems {
		for _, partItem := range partItems {
			if partItem.Leader == nil {
				continue
			}
			if partItem.Leader.Host == podFQDN {
				newLeader := getNewLeader(nc, *nc.Spec.Storaged.Replicas, ordinal)
				if err := s.transLeader(sc, nc, spaceID, partItem.PartID, newLeader); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *storagedUpdater) transLeader(
	storageClient nebula.StorageInterface,
	nc *v1alpha1.NebulaCluster,
	spaceID ng.GraphSpaceID,
	partID ng.PartitionID,
	newLeader *ng.HostAddr,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if err := storageClient.TransLeader(spaceID, partID, newLeader); err != nil {
		return err
	}
	log.Info("transfer leader successfully", "space", spaceID, "partition", partID)
	return nil
}

func (s *storagedUpdater) updateRunningPhase(mc nebula.MetaInterface, nc *v1alpha1.NebulaCluster, spaces []*meta.IdName) error {
	if len(spaces) == 0 {
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
		return nil
	}

	for _, space := range spaces {
		if err := mc.BalanceLeader(space.Name); err != nil {
			return err
		}
	}

	nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	return nil
}

func getNewLeader(nc *v1alpha1.NebulaCluster, replicas, ordinal int32) *ng.HostAddr {
	var podFQDN string
	newLeader := &ng.HostAddr{
		Port: nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift),
	}

	if replicas == 3 || replicas > 3 && replicas&1 == 0 {
		podFQDN = nc.StoragedComponent().GetPodFQDN((ordinal + 2) % replicas)
		newLeader.Host = podFQDN
	} else if replicas > 3 && replicas&1 == 1 {
		podFQDN = nc.StoragedComponent().GetPodFQDN((ordinal + 3) % replicas)
		newLeader.Host = podFQDN
	}

	return newLeader
}
