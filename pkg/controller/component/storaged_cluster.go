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

	nebulago "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type storagedCluster struct {
	clientSet     kube.ClientSet
	dm            discovery.Interface
	scaleManager  ScaleManager
	updateManager UpdateManager
}

func NewStoragedCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	sm ScaleManager,
	um UpdateManager,
) ReconcileManager {
	return &storagedCluster{
		clientSet:     clientSet,
		dm:            dm,
		scaleManager:  sm,
		updateManager: um,
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
	newSvc := nc.StoragedComponent().GenerateService()
	if newSvc == nil {
		return nil
	}

	return syncService(newSvc, c.clientSet.Service())
}

func (c *storagedCluster) syncStoragedWorkload(nc *v1alpha1.NebulaCluster) error {
	namespace := nc.GetNamespace()
	componentName := nc.StoragedComponent().GetName()

	gvk, err := resource.GetGVKFromDefinition(c.dm, nc.Spec.Reference)
	if err != nil {
		return fmt.Errorf("get workload kind failed: %v", err)
	}

	oldWorkloadTemp, err := c.clientSet.Workload().GetWorkload(namespace, componentName, gvk)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("get storaged cluster failed: %v", err)
		return err
	}

	notExist := apierrors.IsNotFound(err)
	oldWorkload := oldWorkloadTemp.DeepCopy()

	cm, cmHash, e, err := c.syncStoragedConfigMap(nc.DeepCopy())
	if err != nil {
		return err
	}

	newWorkload, err := nc.StoragedComponent().GenerateWorkload(gvk, cm)
	if err != nil {
		klog.Errorf("generate storaged cluster template failed: %v", err)
		return err
	}

	if err := extender.SetTemplateAnnotations(
		newWorkload,
		map[string]string{annotation.AnnPodConfigMapHash: cmHash}); err != nil {
		return err
	}

	if err := c.syncNebulaClusterStatus(nc, oldWorkload); err != nil {
		return fmt.Errorf("sync storaged cluster status failed: %v", err)
	}

	if notExist {
		if err := extender.SetLastAppliedConfigAnnotation(newWorkload); err != nil {
			return err
		}
		if err := c.clientSet.Workload().CreateWorkload(newWorkload); err != nil {
			return err
		}
		nc.Status.Storaged.Workload = v1alpha1.WorkloadStatus{}
		return utilerrors.ReconcileErrorf("waiting for storaged cluster [%s/%s] running", namespace, componentName)
	}

	oldReplicas := extender.GetReplicas(oldWorkload)
	newReplicas := extender.GetReplicas(newWorkload)
	if !nc.Status.Storaged.HostsAdded {
		if err := c.addStorageHosts(nc, *oldReplicas, *newReplicas); err != nil {
			return err
		}
		nc.Status.Storaged.HostsAdded = true
	}

	if *newReplicas > *oldReplicas {
		if err := c.addStorageHosts(nc, *oldReplicas, *newReplicas); err != nil {
			klog.Errorf("add storage hosts failed: %v", err)
			return err
		}
		klog.Infof("storaged cluster [%s/%s] add hosts succeed", namespace, componentName)
	}
	if err := c.scaleManager.Scale(nc, oldWorkload, newWorkload); err != nil {
		klog.Errorf("scale storaged cluster [%s/%s] failed: %v", namespace, componentName, err)
		return err
	}

	if !extender.PodTemplateEqual(newWorkload, oldWorkload) ||
		nc.Status.Storaged.Phase == v1alpha1.UpdatePhase {
		oldVolumeClaims := extender.GetDataVolumeClaims(oldWorkload)
		newVolumeClaims := extender.GetDataVolumeClaims(newWorkload)
		if len(oldVolumeClaims) != len(newVolumeClaims) {
			return fmt.Errorf("update storage data volume claims is not supported")
		}
		if err := c.updateManager.Update(nc, oldWorkload, newWorkload, gvk); err != nil {
			return err
		}
	}

	if nc.StoragedComponent().IsReady() {
		endpoints := nc.GetStoragedEndpoints(v1alpha1.StoragedPortNameHTTP)
		if err := updateDynamicFlags(endpoints, newWorkload.GetAnnotations(), oldWorkload.GetAnnotations(), e); err != nil {
			return fmt.Errorf("update storaged cluster %s dynamic flags failed: %v", newWorkload.GetName(), err)
		}
	}

	return extender.UpdateWorkload(c.clientSet.Workload(), newWorkload, oldWorkload)
}

func (c *storagedCluster) syncNebulaClusterStatus(
	nc *v1alpha1.NebulaCluster,
	oldWorkload *unstructured.Unstructured,
) error {
	if oldWorkload == nil {
		return nil
	}

	if nc.Status.Storaged.Phase == "" {
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	}

	updating, err := isUpdating(nc.StoragedComponent(), c.clientSet.Pod(), oldWorkload)
	if err != nil {
		return err
	}

	if updating && nc.Status.Metad.Phase != v1alpha1.UpdatePhase {
		nc.Status.Storaged.Phase = v1alpha1.UpdatePhase
	}

	// TODO: show storaged hosts state with storaged peers
	return syncComponentStatus(nc.StoragedComponent(), &nc.Status.Storaged.ComponentStatus, oldWorkload)
}

func (c *storagedCluster) syncStoragedConfigMap(nc *v1alpha1.NebulaCluster) (*corev1.ConfigMap, string, bool, error) {
	return syncConfigMap(
		nc.StoragedComponent(),
		c.clientSet.ConfigMap(),
		v1alpha1.StoragedConfigTemplate,
		nc.StoragedComponent().GetConfigMapKey())
}

func (c *storagedCluster) addStorageHosts(nc *v1alpha1.NebulaCluster, oldReplicas, newReplicas int32) error {
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
		_ = metaClient.Disconnect()
	}()

	var start int32
	if newReplicas > oldReplicas {
		start = oldReplicas
	}

	port := nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift)
	hosts := make([]*nebulago.HostAddr, 0, newReplicas-start)
	for i := start; i < newReplicas; i++ {
		hosts = append(hosts, &nebulago.HostAddr{
			Host: nc.StoragedComponent().GetPodFQDN(i),
			Port: port,
		})
	}

	return metaClient.AddHosts(hosts)
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
