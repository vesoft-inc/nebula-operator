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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	errorutils "k8s.io/apimachinery/pkg/util/errors"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component/reclaimer"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

type ControlInterface interface {
	UpdateNebulaCluster(cluster *v1alpha1.NebulaCluster) error
}

var _ ControlInterface = &defaultNebulaClusterControl{}

func NewDefaultNebulaClusterControl(
	nebulaClient kube.NebulaCluster,
	graphdCluster component.ReconcileManager,
	metadCluster component.ReconcileManager,
	storagedCluster component.ReconcileManager,
	metaReconciler component.ReconcileManager,
	pvcReclaimer reclaimer.PVCReclaimer,
	conditionUpdater ClusterConditionUpdater) ControlInterface {
	return &defaultNebulaClusterControl{
		nebulaClient:     nebulaClient,
		graphdCluster:    graphdCluster,
		metadCluster:     metadCluster,
		storagedCluster:  storagedCluster,
		metaReconciler:   metaReconciler,
		pvcReclaimer:     pvcReclaimer,
		conditionUpdater: conditionUpdater,
	}
}

type defaultNebulaClusterControl struct {
	nebulaClient     kube.NebulaCluster
	graphdCluster    component.ReconcileManager
	metadCluster     component.ReconcileManager
	storagedCluster  component.ReconcileManager
	metaReconciler   component.ReconcileManager
	pvcReclaimer     reclaimer.PVCReclaimer
	conditionUpdater ClusterConditionUpdater
}

func (c *defaultNebulaClusterControl) UpdateNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	var errs []error
	oldStatus := nc.Status.DeepCopy()

	if err := c.updateNebulaCluster(nc); err != nil {
		errs = append(errs, err)
	}

	c.conditionUpdater.Update(nc)

	if apiequality.Semantic.DeepEqual(&nc.Status, oldStatus) {
		return errorutils.NewAggregate(errs)
	}

	if err := c.nebulaClient.UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		errs = append(errs, err)
	}

	return errorutils.NewAggregate(errs)
}

func (c *defaultNebulaClusterControl) updateNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	log := getLog().WithValues("namespace", nc.Namespace, "name", nc.Name)
	if err := c.metadCluster.Reconcile(nc); err != nil {
		log.Error(err, "reconcile metad cluster failed")
		return err
	}

	if err := c.storagedCluster.Reconcile(nc); err != nil {
		log.Error(err, "reconcile storaged cluster failed")
		return err
	}

	if err := c.graphdCluster.Reconcile(nc); err != nil {
		log.Error(err, "reconcile graphd cluster failed")
		return err
	}

	if err := c.metaReconciler.Reconcile(nc); err != nil {
		log.Error(err, "reconcile pv and pvc metadata cluster failed")
		return err
	}

	if err := c.pvcReclaimer.Reclaim(nc); err != nil {
		log.Error(err, "reclaim pvc failed")
		return err
	}

	return nil
}

func (c *defaultNebulaClusterControl) resetImage(nc *v1alpha1.NebulaCluster) {
	if nc.Status.Graphd.Version != "" {
		nc.Spec.Graphd.Version = nc.Status.Graphd.Version
	}
	if nc.Status.Metad.Version != "" {
		nc.Spec.Metad.Version = nc.Status.Metad.Version
	}
	if nc.Status.Storaged.Version != "" {
		nc.Spec.Storaged.Version = nc.Status.Storaged.Version
	}
}

type FakeClusterControl struct {
	err error
}

func NewFakeClusterControl() *FakeClusterControl {
	return &FakeClusterControl{}
}

func (f *FakeClusterControl) SetUpdateNebulaClusterError(err error) {
	f.err = err
}

func (f *FakeClusterControl) UpdateNebulaCluster(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
