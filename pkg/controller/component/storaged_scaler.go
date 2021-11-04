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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nebulago "github.com/vesoft-inc/nebula-go/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
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
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	nc.Status.Storaged.Phase = v1alpha1.ScaleOutPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	if !nc.StoragedComponent().IsReady() {
		log.Info("storage cluster status not ready", "storage", nc.StoragedComponent().GetName())
		return nil
	}

	if pointer.BoolPtrDerefOr(nc.Spec.Storaged.EnableAutoBalance, false) {
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
		return nil
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints)
	if err != nil {
		log.Error(err, "create meta client failed", "endpoints", endpoints)
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			log.Error(err, "disconnect meta client failed", "endpoints", endpoints)
		}
	}()

	if err := metaClient.BalanceData(); err != nil {
		return err
	}

	if err := metaClient.BalanceLeader(); err != nil {
		log.Error(err, "unable to balance leader")
		return err
	}

	nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	return nil
}

func (ss *storageScaler) ScaleIn(nc *v1alpha1.NebulaCluster, oldReplicas, newReplicas int32) error {
	log := getLog().WithValues("namespace", nc.GetNamespace(), "name", nc.GetName())
	nc.Status.Storaged.Phase = v1alpha1.ScaleInPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints)
	if err != nil {
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			log.Error(err, "meta client disconnect", "endpoints", endpoints)
		}
	}()

	if oldReplicas-newReplicas > 0 {
		hosts := make([]*nebulago.HostAddr, 0, oldReplicas-newReplicas)
		port := nebulago.Port(nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift))
		for i := oldReplicas - 1; i >= newReplicas; i-- {
			hosts = append(hosts, &nebulago.HostAddr{
				Host: nc.StoragedComponent().GetPodFQDN(i),
				Port: port,
			})
		}
		if len(hosts) > 0 {
			if err := metaClient.RemoveHost(hosts); err != nil {
				return err
			}
		}
	}

	if err := PvcMark(ss.clientSet.PVC(), nc.StoragedComponent(), oldReplicas, newReplicas); err != nil {
		return err
	}

	var deleted bool
	pvcName := ordinalPVCName(nc.StoragedComponent().Type(), nc.StoragedComponent().GetName(), newReplicas)
	_, err = ss.clientSet.PVC().GetPVC(nc.GetNamespace(), pvcName)
	if apierrors.IsNotFound(err) {
		deleted = true
	}

	if deleted && nc.StoragedComponent().IsReady() {
		log.Info("all used pvcs were reclaimed", "storage", nc.StoragedComponent().GetName())
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	}

	return nil
}
