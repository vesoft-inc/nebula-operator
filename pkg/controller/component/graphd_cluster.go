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
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type graphdCluster struct {
	clientSet            kube.ClientSet
	dm                   discovery.Interface
	updateManager        UpdateManager
	enableEvenPodsSpread bool
}

func NewGraphdCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	um UpdateManager,
	enableEvenPodsSpread bool) ReconcileManager {
	return &graphdCluster{
		clientSet:            clientSet,
		dm:                   dm,
		updateManager:        um,
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
	componentName := nc.GraphdComponent().GetName()

	gvk, err := resource.GetGVKFromDefinition(c.dm, nc.Spec.Reference)
	if err != nil {
		return fmt.Errorf("get workload kind failed: %v", err)
	}

	oldWorkloadTemp, err := c.clientSet.Workload().GetWorkload(namespace, componentName, gvk)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("get graphd cluster failed: %v", err)
		return err
	}

	notExist := apierrors.IsNotFound(err)
	oldWorkload := oldWorkloadTemp.DeepCopy()

	cm, cmHash, e, err := c.syncGraphdConfigMap(nc)
	if err != nil {
		return err
	}

	newWorkload, err := nc.GraphdComponent().GenerateWorkload(gvk, cm, c.enableEvenPodsSpread)
	if err != nil {
		klog.Errorf("generate graphd cluster template failed: %v", err)
		return err
	}

	if err := extender.SetTemplateAnnotations(
		newWorkload,
		map[string]string{annotation.AnnPodConfigMapHash: cmHash}); err != nil {
		return err
	}

	if err := c.syncNebulaClusterStatus(nc, newWorkload, oldWorkload); err != nil {
		return fmt.Errorf("sync graphd cluster status status failed: %v", err)
	}

	if notExist {
		if err := extender.SetLastAppliedConfigAnnotation(newWorkload); err != nil {
			return err
		}
		if err := c.clientSet.Workload().CreateWorkload(newWorkload); err != nil {
			return err
		}
		nc.Status.Graphd.Workload = v1alpha1.WorkloadStatus{}
		return utilerrors.ReconcileErrorf("waiting for graphd cluster %s running", newWorkload.GetName())
	}

	if !extender.PodTemplateEqual(newWorkload, oldWorkload) ||
		nc.Status.Graphd.Phase == v1alpha1.UpdatePhase {
		if err := c.updateManager.Update(nc, oldWorkload, newWorkload, gvk); err != nil {
			return err
		}
	}

	if nc.GraphdComponent().IsReady() {
		endpoints := nc.GetGraphdEndpoints(v1alpha1.GraphdPortNameHTTP)
		if err := updateDynamicFlags(endpoints, newWorkload.GetAnnotations(), oldWorkload.GetAnnotations(), e); err != nil {
			return fmt.Errorf("update graphd cluster %s dynamic flags failed: %v", newWorkload.GetName(), err)
		}
	}

	return extender.UpdateWorkload(c.clientSet.Workload(), newWorkload, oldWorkload)
}

func (c *graphdCluster) syncNebulaClusterStatus(
	nc *v1alpha1.NebulaCluster,
	newWorkload,
	oldWorkload *unstructured.Unstructured) error {
	if oldWorkload == nil {
		return nil
	}

	oldReplicas := extender.GetReplicas(oldWorkload)
	newReplicas := extender.GetReplicas(newWorkload)
	updating, err := isUpdating(nc.GraphdComponent(), c.clientSet.Pod(), oldWorkload)
	if err != nil {
		return err
	}

	if updating &&
		nc.Status.Metad.Phase != v1alpha1.UpdatePhase {
		nc.Status.Graphd.Phase = v1alpha1.UpdatePhase
	} else if *newReplicas < *oldReplicas {
		nc.Status.Graphd.Phase = v1alpha1.ScaleInPhase
		if err := PvcMark(c.clientSet.PVC(), nc.GraphdComponent(), *oldReplicas, *newReplicas); err != nil {
			return err
		}
	} else if *newReplicas > *oldReplicas {
		nc.Status.Graphd.Phase = v1alpha1.ScaleOutPhase
	} else {
		nc.Status.Graphd.Phase = v1alpha1.RunningPhase
	}

	return syncComponentStatus(nc.GraphdComponent(), &nc.Status.Graphd, oldWorkload)
}

func (c *graphdCluster) syncGraphdService(nc *v1alpha1.NebulaCluster) error {
	newSvc := nc.GraphdComponent().GenerateService()
	if newSvc == nil {
		return nil
	}

	return syncService(newSvc, c.clientSet.Service())
}

func (c *graphdCluster) syncGraphdConfigMap(nc *v1alpha1.NebulaCluster) (*corev1.ConfigMap, string, bool, error) {
	return syncConfigMap(
		nc.GraphdComponent(),
		c.clientSet.ConfigMap(),
		v1alpha1.GraphdConfigTemplate,
		nc.GraphdComponent().GetConfigMapKey())
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
