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
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component/reclaimer"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

type ControlInterface interface {
	UpdateNebulaCluster(cluster *v1alpha1.NebulaCluster) error
}

var _ ControlInterface = &defaultNebulaClusterControl{}

func NewDefaultNebulaClusterControl(
	client client.Client,
	nebulaClient kube.NebulaCluster,
	graphdCluster component.ReconcileManager,
	metadCluster component.ReconcileManager,
	storagedCluster component.ReconcileManager,
	exporter component.ReconcileManager,
	console component.ReconcileManager,
	metaReconciler component.ReconcileManager,
	pvcReclaimer reclaimer.PVCReclaimer,
	conditionUpdater ClusterConditionUpdater,
) ControlInterface {
	return &defaultNebulaClusterControl{
		client:           client,
		nebulaClient:     nebulaClient,
		graphdCluster:    graphdCluster,
		metadCluster:     metadCluster,
		storagedCluster:  storagedCluster,
		exporter:         exporter,
		console:          console,
		metaReconciler:   metaReconciler,
		pvcReclaimer:     pvcReclaimer,
		conditionUpdater: conditionUpdater,
	}
}

type defaultNebulaClusterControl struct {
	client           client.Client
	nebulaClient     kube.NebulaCluster
	graphdCluster    component.ReconcileManager
	metadCluster     component.ReconcileManager
	storagedCluster  component.ReconcileManager
	exporter         component.ReconcileManager
	console          component.ReconcileManager
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

	if apiequality.Semantic.DeepEqual(&nc.Status, oldStatus) && nc.IsConditionReady() {
		return errorutils.NewAggregate(errs)
	}

	nc.Status.ObservedGeneration = nc.Generation
	if err := c.nebulaClient.UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		errs = append(errs, err)
	}

	if !nc.IsConditionReady() {
		errs = append(errs, utilerrors.ReconcileErrorf("waiting for nebulacluster ready"))
	}

	return errorutils.NewAggregate(errs)
}

func (c *defaultNebulaClusterControl) updateNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	if err := kube.CheckRBAC(context.TODO(), c.client, nc.Namespace); err != nil {
		return err
	}

	if err := c.metadCluster.Reconcile(nc); err != nil {
		klog.Errorf("reconcile metad cluster failed: %v", err)
		return err
	}

	if nc.IsBREnabled() &&
		annotation.IsRestoreNameNotEmpty(nc.GetAnnotations()) &&
		!annotation.IsRestoreMetadDone(nc.GetAnnotations()) {
		return utilerrors.ReconcileErrorf("waiting for metad cluster restore done")
	}

	if err := c.storagedCluster.Reconcile(nc); err != nil {
		klog.Errorf("reconcile storaged cluster failed: %v", err)
		return err
	}

	if nc.IsBREnabled() &&
		annotation.IsRestoreNameNotEmpty(nc.GetAnnotations()) &&
		!annotation.IsRestoreStoragedDone(nc.GetAnnotations()) {
		return utilerrors.ReconcileErrorf("waiting for storaged cluster restore done")
	}

	if err := c.graphdCluster.Reconcile(nc); err != nil {
		klog.Errorf("reconcile graphd cluster failed: %v", err)
		return err
	}

	if err := c.exporter.Reconcile(nc); err != nil {
		klog.Errorf("reconcile exporter failed: %v", err)
		return err
	}

	if err := c.console.Reconcile(nc); err != nil {
		klog.Errorf("reconcile console failed: %v", err)
		return err
	}

	klog.Infof("start reconcile pv and pvc metadata cluster")
	if err := c.metaReconciler.Reconcile(nc); err != nil {
		klog.Errorf("reconcile pv and pvc metadata cluster failed: %v", err)
		return err
	}
	klog.Infof("finished reconcile pv and pvc metadata cluster successfully")

	if err := c.pvcReclaimer.Reclaim(nc); err != nil {
		klog.Errorf("reclaim pvc failed: %v", err)
		return err
	}

	return nil
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
