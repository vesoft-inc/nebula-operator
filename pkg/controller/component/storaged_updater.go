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

	"github.com/facebook/fbthrift/thrift/lib/go/thrift"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ng "github.com/vesoft-inc/nebula-go/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	extenderutil "github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

const (
	// TransLeaderBeginTime is the key of trans Leader begin time
	TransLeaderBeginTime = "transLeaderBeginTime"
	// TransLeaderTimeout is the timeout limit of trans leader
	TransLeaderTimeout = 3 * time.Minute
)

type storagedUpdate struct {
	client.Client
	clientSet kube.ClientSet
	extender  extenderutil.UnstructuredExtender
}

func NewStoragedUpdater(cli client.Client, clientSet kube.ClientSet) UpdateManager {
	return &storagedUpdate{
		Client:    cli,
		clientSet: clientSet,
		extender:  extenderutil.New(),
	}
}

func (s *storagedUpdate) Update(
	nc *v1alpha1.NebulaCluster,
	oldUnstruct, newUnstruct *unstructured.Unstructured,
	gvk schema.GroupVersionKind,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if *nc.Spec.Storaged.Replicas == int32(0) {
		return nil
	}

	if nc.Status.Storaged.Phase == v1alpha1.ScaleInPhase || nc.Status.Storaged.Phase == v1alpha1.ScaleOutPhase {
		log.Info("storaged is scaling, can not update")
		return getLastConfig(s.extender, oldUnstruct, newUnstruct)
	}

	nc.Status.Storaged.Phase = v1alpha1.UpdatePhase
	if !extenderutil.PodTemplateEqual(s.extender, newUnstruct, oldUnstruct) {
		return nil
	}

	if nc.Status.Storaged.Workload.UpdateRevision == nc.Status.Storaged.Workload.CurrentRevision {
		return nil
	}

	spec := s.extender.GetSpec(oldUnstruct)
	actualStrategy := spec["updateStrategy"].(map[string]interface{})
	advanced := gvk.Kind == resource.AdvancedStatefulSetKind.Kind
	partition := actualStrategy["rollingUpdate"].(map[string]interface{})
	if err := setPartition(s.extender, newUnstruct, partition["partition"].(int64), advanced); err != nil {
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

	replicas := s.extender.GetReplicas(oldUnstruct)
	index, err := getNextUpdatePod(nc.StoragedComponent(), *replicas, s.clientSet.Pod())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return utilerrors.ReconcileErrorf("%v", err)
		}
		return err
	}
	if index >= 0 {
		return s.updateStoragedPod(mc, nc, index, newUnstruct, advanced)
	}

	if err := mc.BalanceLeader(); err != nil {
		log.Error(err, "balance leader failed")
		return err
	}

	hostItem, err := mc.ListHosts()
	if err != nil {
		return err
	}
	if !mc.IsBalanced(hostItem) {
		if err := mc.BalanceLeader(); err != nil {
			log.Error(err, "rebalance leader failed")
			return err
		}
	}

	nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	return nil
}

func (s *storagedUpdate) updateStoragedPod(
	mc nebula.MetaInterface,
	nc *v1alpha1.NebulaCluster,
	ordinal int32,
	newUnstruct *unstructured.Unstructured,
	advanced bool,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if *nc.Spec.Storaged.Replicas == 1 {
		return setPartition(s.extender, newUnstruct, int64(ordinal), advanced)
	}

	namespace, ncName := nc.GetNamespace(), nc.GetName()
	updatePodName := nc.StoragedComponent().GetPodName(ordinal)
	updatePod, err := s.clientSet.Pod().GetPod(namespace, updatePodName)
	if err != nil {
		log.Error(err, "failed to get pod")
		return err
	}

	spaces, err := mc.ListSpaces()
	if err != nil {
		return err
	}

	if len(spaces) == 0 {
		return setPartition(s.extender, newUnstruct, int64(ordinal), advanced)
	}

	if updatePod.Annotations == nil {
		updatePod.Annotations = map[string]string{}
	}
	now := time.Now().Format(time.RFC3339)
	updatePod.Annotations[TransLeaderBeginTime] = now

	transAddr := nc.StoragedComponent().GetPodConnAddresses(v1alpha1.StoragedPortNameThrift, ordinal)
	if err := s.transLeaderIfNecessary(nc, mc, transAddr, updatePod); err != nil {
		return err
	}

	if err := s.clientSet.Pod().UpdatePod(updatePod); err != nil {
		return err
	}

	if s.readyToUpdate(mc, transAddr, updatePod) {
		return setPartition(s.extender, newUnstruct, int64(ordinal), advanced)
	}
	return fmt.Errorf("%s/%s's storaged pod: %s is transferring leader", namespace, ncName, updatePodName)
}

func (s *storagedUpdate) readyToUpdate(mc nebula.MetaInterface, leaderHost string, updatePod *corev1.Pod) bool {
	log := getLog().WithValues("updatePod", updatePod.GetName())
	count, err := mc.GetLeaderCount(leaderHost)
	if err != nil {
		log.Error(err, "get leader count failed")
		return false
	}
	if count == 0 {
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

func (s *storagedUpdate) transLeaderIfNecessary(
	nc *v1alpha1.NebulaCluster,
	mc nebula.MetaInterface,
	transAddr string,
	updatePod *corev1.Pod,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	sc, err := s.getReadyStorageClient(nc)
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
			if partItem.Leader.Host == transAddr {
				newLeader := getNewLeader(partItem.Leader, partItem.Peers)
				if err := s.transLeader(sc, nc, spaceID, partItem.PartID, newLeader, updatePod); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *storagedUpdate) transLeader(
	storageClient nebula.StorageInterface,
	nc *v1alpha1.NebulaCluster,
	spaceID ng.GraphSpaceID,
	partID ng.PartitionID,
	newLeader *ng.HostAddr,
	pod *corev1.Pod,
) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	if err := storageClient.TransLeader(spaceID, partID, newLeader); err != nil {
		return err
	}
	log.Info("transfer leader successfully", "space", spaceID, "partition", partID,
		"pod", pod.GetName())
	return nil
}

func (s *storagedUpdate) getReadyStorageClient(nc *v1alpha1.NebulaCluster) (nebula.StorageInterface, error) {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	namespace := nc.GetNamespace()
	storageSvcName := nc.StoragedComponent().GetServiceName()

	storageEndpoint, err := s.clientSet.Endpoint().GetEndpoints(namespace, storageSvcName)
	if err != nil {
		log.Error(err, "get storaged endpoints failed")
		return nil, err
	}

	eps := getReadyAddress(storageEndpoint)
	sc, err := nebula.NewStorageClient(eps)
	if err != nil {
		transErr, ok := err.(thrift.TransportException)
		if !ok {
			return nil, err
		}
		if transErr.TypeID() != thrift.NOT_OPEN {
			return nil, transErr
		}
		var retryErr error
		for retry := 0; retry < len(eps); retry++ {
			// remove unreachable endpoint
			eps = append(eps[:0], eps[1:]...)
			sc, retryErr = nebula.NewStorageClient(eps)
			if retryErr == nil {
				break
			}
		}
	}
	return sc, nil
}

func getNewLeader(leader *ng.HostAddr, peers []*ng.HostAddr) *ng.HostAddr {
	others := make([]*ng.HostAddr, 0)
	for _, peer := range peers {
		if leader.Host == peer.Host {
			continue
		}
		others = append(others, peer)
		return others[rand.Intn(len(others))]
	}
	return nil
}

func getReadyAddress(endpoint *corev1.Endpoints) []string {
	eps := make([]string, 0)
	for _, subset := range endpoint.Subsets {
		for _, addr := range subset.Addresses {
			eps = append(eps, addr.IP+":9778")
		}
	}
	return eps
}
