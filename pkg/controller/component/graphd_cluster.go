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

type graphdCluster struct {
	clientSet            kube.ClientSet
	dm                   discovery.Interface
	extender             controllerutil.UnstructuredExtender
	enableEvenPodsSpread bool
}

func NewGraphdCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	enableEvenPodsSpread bool) ReconcileManager {
	return &graphdCluster{
		clientSet:            clientSet,
		dm:                   dm,
		extender:             &controllerutil.Unstructured{},
		enableEvenPodsSpread: enableEvenPodsSpread,
	}
}

func (c *graphdCluster) Reconcile(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Graphd == nil {
		return nil
	}

	if err := c.syncGraphdService(nc); err != nil {
		return err
	}
	return c.syncGraphdWorkload(nc)
}

func (c *graphdCluster) syncGraphdWorkload(nc *v1alpha1.NebulaCluster) error {
	namespace := nc.GetNamespace()
	ncName := nc.GetName()
	componentName := nc.GraphdComponent().GetName()

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

	cm, cmHash, err := c.syncGraphdConfigMap(nc)
	if err != nil {
		return err
	}

	newWorkload, err := nc.GraphdComponent().GenerateWorkload(gvk, cm, c.enableEvenPodsSpread)
	if err != nil {
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
		nc.Status.Graphd.Workload = v1alpha1.WorkloadStatus{}
		return utilerrors.ReconcileErrorf("waiting for graphd cluster %s running", newWorkload.GetName())
	}
	nc.Status.Graphd.Phase = v1alpha1.RunningPhase

	oldReplicas := c.extender.GetReplicas(oldWorkload)
	newReplicas := c.extender.GetReplicas(newWorkload)
	if *oldReplicas > *newReplicas {
		nc.Status.Graphd.Phase = v1alpha1.ScaleInPhase
		if err := PvcMark(c.clientSet.PVC(), nc.GraphdComponent(), *oldReplicas, *newReplicas); err != nil {
			return err
		}
	}
	if *oldReplicas < *newReplicas {
		nc.Status.Graphd.Phase = v1alpha1.ScaleOutPhase
	}

	return controllerutil.UpdateWorkload(c.clientSet.Workload(), c.extender, newWorkload, oldWorkload)
}

func (c *graphdCluster) syncNebulaClusterStatus(nc *v1alpha1.NebulaCluster, workload *unstructured.Unstructured) error {
	return syncComponentStatus(nc.GraphdComponent(), c.extender, &nc.Status.Graphd, workload)
}

func (c *graphdCluster) syncGraphdService(nc *v1alpha1.NebulaCluster) error {
	return syncService(nc.GraphdComponent(), c.clientSet.Service())
}

func (c *graphdCluster) syncGraphdConfigMap(nc *v1alpha1.NebulaCluster) (*corev1.ConfigMap, string, error) {
	return syncConfigMap(
		nc.GraphdComponent(),
		c.clientSet.ConfigMap(),
		v1alpha1.GraphdConfigTemplate,
		nc.GraphdComponent().GetConfigMapKey(),
	)
}

type FakeGraphdCluster struct {
	err error
}

func NewFakeGraphdCluster() *FakeGraphdCluster {
	return &FakeGraphdCluster{}
}

func (f *FakeGraphdCluster) SetReconcileError(err error) {
	f.err = err
}

func (f *FakeGraphdCluster) Reconcile(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
