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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	controllerutil "github.com/vesoft-inc/nebula-operator/pkg/util/controller"
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type storagedCluster struct {
	clientSet            kube.ClientSet
	dm                   discovery.Interface
	extender             controllerutil.UnstructuredExtender
	scaleManager         ScaleManager
	enableEvenPodsSpread bool
}

func NewStoragedCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	sm ScaleManager,
	enableEvenPodsSpread bool) ReconcileManager {
	return &storagedCluster{
		clientSet:            clientSet,
		dm:                   dm,
		extender:             &controllerutil.Unstructured{},
		scaleManager:         sm,
		enableEvenPodsSpread: enableEvenPodsSpread,
	}
}

func (c *storagedCluster) Reconcile(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Storaged == nil {
		return nil
	}

	if err := c.syncStoragedHeadlessService(nc); err != nil {
		return err
	}

	return c.syncStoragedWorkload(nc)
}

func (c *storagedCluster) syncStoragedHeadlessService(nc *v1alpha1.NebulaCluster) error {
	return syncService(nc.StoragedComponent(), c.clientSet.Service())
}

func (c *storagedCluster) syncStoragedWorkload(nc *v1alpha1.NebulaCluster) error {
	namespace := nc.GetNamespace()
	ncName := nc.GetName()
	componentName := nc.StoragedComponent().GetName()

	gvk, err := resource.GetGVKFromDefinition(c.dm, nc.Spec.Reference)
	if err != nil {
		return fmt.Errorf("get workload reference error: %v", err)
	}

	oldWorkloadTemp, err := c.clientSet.Workload().GetWorkload(namespace, componentName, gvk)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("failed to get workload %s for cluster %s/%s, error: %s", componentName, namespace, ncName, err)
		return err
	}

	notExist := apierrors.IsNotFound(err)

	oldWorkload := oldWorkloadTemp.DeepCopy()
	if err := c.syncNebulaClusterStatus(nc, oldWorkload); err != nil {
		klog.Errorf("failed to sync %s/%s's status, error: %v", namespace, ncName, err)
		return err
	}

	cm, cmHash, err := c.syncStoragedConfigMap(nc)
	if err != nil {
		return err
	}

	newWorkload, err := nc.StoragedComponent().GenerateWorkload(gvk, cm, c.enableEvenPodsSpread)
	if err != nil {
		klog.Errorf("generate workload template failed: %v", err)
		return err
	}
	if err := c.extender.SetTemplateAnnotations(
		newWorkload,
		map[string]string{annotation.AnnPodConfigMapHash: cmHash}); err != nil {
		return err
	}

	if notExist {
		if err := controllerutil.SetLastAppliedConfigAnnotation(c.extender, newWorkload); err != nil {
			return err
		}
		if err := c.clientSet.Workload().CreateWorkload(newWorkload); err != nil {
			return err
		}
		nc.Status.Storaged.Workload = v1alpha1.WorkloadStatus{}
		return utilerrors.ReconcileErrorf("waiting for storaged cluster %s running", newWorkload.GetName())
	}
	if nc.Status.Storaged.Phase == "" {
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	}
	if err := c.scaleManager.Scale(nc, oldWorkload, newWorkload); err != nil {
		klog.Errorf("failed to scale cluster %s/%s, error: %v", namespace, ncName, err)
		return err
	}

	return controllerutil.UpdateWorkload(c.clientSet.Workload(), c.extender, newWorkload, oldWorkload)
}

func (c *storagedCluster) syncNebulaClusterStatus(nc *v1alpha1.NebulaCluster, workload *unstructured.Unstructured) error {
	// TODO: show storaged hosts state with storaged peers
	return syncComponentStatus(nc.StoragedComponent(), c.extender, &nc.Status.Storaged, workload)
}

func (c *storagedCluster) syncStoragedConfigMap(nc *v1alpha1.NebulaCluster) (*corev1.ConfigMap, string, error) {
	return syncConfigMap(nc.StoragedComponent(), c.clientSet.ConfigMap(), v1alpha1.StoragedConfigTemplate,
		nc.StoragedComponent().GetConfigMapKey())
}

type FakeStoragedCluster struct {
	err error
}

func NewFakeStoragedCluster() *FakeStoragedCluster {
	return &FakeStoragedCluster{}
}

func (f *FakeStoragedCluster) SetReconcileError(err error) {
	f.err = err
}

func (f *FakeStoragedCluster) Reconcile(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
