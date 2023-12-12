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

	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type metadCluster struct {
	clientSet     kube.ClientSet
	dm            discovery.Interface
	updateManager UpdateManager
}

func NewMetadCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	um UpdateManager,
) ReconcileManager {
	return &metadCluster{
		clientSet:     clientSet,
		dm:            dm,
		updateManager: um,
	}
}

func (c *metadCluster) Reconcile(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Metad == nil {
		return nil
	}

	if err := c.syncMetadHeadlessService(nc); err != nil {
		return err
	}

	return c.syncMetadWorkload(nc)
}

func (c *metadCluster) Delete(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Metad == nil {
		return nil
	}
	namespace := nc.GetNamespace()
	componentName := nc.MetadComponent().GetName()
	gvk, err := resource.GetGVKFromDefinition(c.dm, nc.Spec.Reference)
	if err != nil {
		return fmt.Errorf("get workload kind failed: %v", err)
	}
	workload, err := c.clientSet.Workload().GetWorkload(namespace, componentName, gvk)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("get metad cluster failed: %v", err)
		return err
	}
	return c.clientSet.Workload().DeleteWorkload(workload)
}

func (c *metadCluster) syncMetadHeadlessService(nc *v1alpha1.NebulaCluster) error {
	newSvc := nc.MetadComponent().GenerateHeadlessService()
	if newSvc == nil {
		return nil
	}

	return syncService(newSvc, c.clientSet.Service())
}

func (c *metadCluster) syncMetadWorkload(nc *v1alpha1.NebulaCluster) error {
	namespace := nc.GetNamespace()
	componentName := nc.MetadComponent().GetName()

	gvk, err := resource.GetGVKFromDefinition(c.dm, nc.Spec.Reference)
	if err != nil {
		return fmt.Errorf("get workload kind failed: %v", err)
	}

	oldWorkloadTemp, err := c.clientSet.Workload().GetWorkload(namespace, componentName, gvk)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("get metad cluster failed: %v", err)
		return err
	}

	notExist := apierrors.IsNotFound(err)
	oldWorkload := oldWorkloadTemp.DeepCopy()

	needSuspend, err := suspendComponent(c.clientSet.Workload(), nc.MetadComponent(), oldWorkload)
	if err != nil {
		return fmt.Errorf("suspend metad cluster %s failed: %v", componentName, err)
	}
	if needSuspend {
		klog.Infof("metad cluster %s is suspended, skip reconciling", componentName)
		return nil
	}

	cm, cmHash, err := c.syncMetadConfigMap(nc.DeepCopy())
	if err != nil {
		return err
	}

	newWorkload, err := nc.MetadComponent().GenerateWorkload(gvk, cm)
	if err != nil {
		klog.Errorf("generate metad cluster template failed: %v", err)
		return err
	}

	if err := extender.SetTemplateAnnotations(
		newWorkload,
		map[string]string{annotation.AnnPodConfigMapHash: cmHash}); err != nil {
		return err
	}

	if !notExist {
		timestamp, ok := oldWorkload.GetAnnotations()[annotation.AnnRestartTimestamp]
		if ok && timestamp != "" {
			if err := extender.SetTemplateAnnotations(newWorkload,
				map[string]string{annotation.AnnRestartTimestamp: timestamp}); err != nil {
				return err
			}
		}
	}

	if err := c.syncNebulaClusterStatus(nc, oldWorkload); err != nil {
		return fmt.Errorf("sync metad cluster status failed: %v", err)
	}

	if notExist {
		if err := extender.SetRestartTimestamp(newWorkload); err != nil {
			return err
		}
		if err := extender.SetLastAppliedConfigAnnotation(newWorkload); err != nil {
			return err
		}
		if err := c.clientSet.Workload().CreateWorkload(newWorkload); err != nil {
			return err
		}
		nc.Status.Metad.Workload = &v1alpha1.WorkloadStatus{}
		return utilerrors.ReconcileErrorf("waiting for metad cluster %s running", newWorkload.GetName())
	}

	equal := extender.PodTemplateEqual(newWorkload, oldWorkload)
	if !equal || nc.Status.Metad.Phase == v1alpha1.UpdatePhase {
		if err := c.updateManager.Update(nc, oldWorkload, newWorkload, gvk); err != nil {
			return err
		}
	}

	if err := c.syncMetadPVC(nc); err != nil {
		return err
	}

	if equal && nc.MetadComponent().IsReady() {
		if err := c.setVersion(nc); err != nil {
			return err
		}

		endpoints := nc.GetMetadEndpoints(v1alpha1.MetadPortNameHTTP)
		if err := updateDynamicFlags(endpoints, newWorkload.GetAnnotations()); err != nil {
			return fmt.Errorf("update metad cluster %s dynamic flags failed: %v", newWorkload.GetName(), err)
		}
	}

	return extender.UpdateWorkload(c.clientSet.Workload(), newWorkload, oldWorkload)
}

func (c *metadCluster) syncNebulaClusterStatus(nc *v1alpha1.NebulaCluster, oldWorkload *unstructured.Unstructured) error {
	if oldWorkload == nil {
		return nil
	}

	updating, err := isUpdating(nc.MetadComponent(), c.clientSet.Pod(), oldWorkload)
	if err != nil {
		return err
	}

	if updating {
		nc.Status.Metad.Phase = v1alpha1.UpdatePhase
	} else {
		nc.Status.Metad.Phase = v1alpha1.RunningPhase
	}

	// TODO: show metad hosts state with metad peers
	return syncComponentStatus(nc.MetadComponent(), &nc.Status.Metad, oldWorkload)
}

func (c *metadCluster) syncMetadConfigMap(nc *v1alpha1.NebulaCluster) (*corev1.ConfigMap, string, error) {
	return syncConfigMap(
		nc.MetadComponent(),
		c.clientSet.ConfigMap(),
		v1alpha1.MetadhConfigTemplate,
		nc.MetadComponent().GetConfigMapKey())
}

func (c *metadCluster) syncMetadPVC(nc *v1alpha1.NebulaCluster) error {
	return syncPVC(nc.MetadComponent(), c.clientSet.PVC())
}

func (c *metadCluster) setVersion(nc *v1alpha1.NebulaCluster) error {
	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
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

	hosts, err := metaClient.ListHosts(meta.ListHostType_META)
	if err != nil {
		return err
	}
	for _, host := range hosts {
		version := host.Version
		nc.Status.Version = string(version)
		break
	}
	return nil
}

type FakeMetadCluster struct {
	err error
}

func NewFakeMetadCluster() *FakeMetadCluster {
	return &FakeMetadCluster{}
}

func (f *FakeMetadCluster) SetReconcileError(err error) {
	f.err = err
}

func (f *FakeMetadCluster) Reconcile(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
