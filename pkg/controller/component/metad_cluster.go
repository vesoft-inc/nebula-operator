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
	"strings"
	"time"

	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type metadCluster struct {
	clientSet       kube.ClientSet
	dm              discovery.Interface
	updateManager   UpdateManager
	failoverManager FailoverManager
	eventRecorder   record.EventRecorder
}

func NewMetadCluster(
	clientSet kube.ClientSet,
	dm discovery.Interface,
	um UpdateManager,
	fm FailoverManager,
	recorder record.EventRecorder,
) ReconcileManager {
	return &metadCluster{
		clientSet:       clientSet,
		dm:              dm,
		updateManager:   um,
		failoverManager: fm,
		eventRecorder:   recorder,
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

	if nc.IsAutoFailoverEnabled() {
		r, hosts, err := c.shouldRecover(nc)
		if err != nil {
			return err
		}
		if r {
			if err := c.failoverManager.Recovery(nc, hosts); err != nil {
				return err
			}
		} else if nc.MetadComponent().IsAutoFailovering() {
			if err := c.failoverManager.Failover(nc); err != nil {
				return err
			}
		}
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
		endpoints := nc.GetMetadEndpoints(v1alpha1.MetadPortNameHTTP)
		if err := updateDynamicFlags(endpoints, oldWorkload.GetAnnotations(), newWorkload.GetAnnotations()); err != nil {
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

	workloadReplicas := getWorkloadReplicas(nc.Status.Metad.Workload)
	if !nc.IsAutoFailoverEnabled() ||
		pointer.Int32Deref(nc.Spec.Metad.Replicas, 0) != workloadReplicas {
		return syncComponentStatus(nc.MetadComponent(), &nc.Status.Metad, oldWorkload)
	}

	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
	if err != nil {
		return err
	}
	hosts := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(hosts, options...)
	if err != nil {
		return err
	}
	defer func() {
		_ = metaClient.Disconnect()
	}()

	hostItems, err := metaClient.ListHosts(meta.ListHostType_META)
	if err != nil {
		return err
	}
	thriftPort := nc.MetadComponent().GetPort(v1alpha1.MetadPortNameThrift)
	for _, host := range hostItems {
		if host.Status == meta.HostStatus_OFFLINE && host.HostAddr.Port == thriftPort {
			podName := strings.Split(host.HostAddr.Host, ".")[0]
			ordinal := getPodOrdinal(podName)
			if int32(ordinal) >= pointer.Int32Deref(nc.Spec.Metad.Replicas, 0) {
				klog.Infof("metad pod [%s/%s] has already been terminated by the sts. Skipping failover and/or removing from auto failover list", nc.Namespace, podName)
				// delete is a no-op if FailureHosts or podName is nil
				delete(nc.Status.Metad.FailureHosts, podName)
				continue
			}
			if nc.Status.Metad.FailureHosts == nil {
				nc.Status.Metad.FailureHosts = make(map[string]v1alpha1.FailureHost)
			}
			fh, exists := nc.Status.Metad.FailureHosts[podName]
			if exists {
				deadline := fh.CreationTime.Add(nc.Spec.FailoverPeriod.Duration)
				if time.Now().After(deadline) {
					if fh.ConfirmationTime.IsZero() {
						fh.ConfirmationTime = metav1.Time{Time: time.Now()}
						cl := label.New().Cluster(nc.GetClusterName()).Metad()
						_, pvcs, err := getPodAndPvcs(c.clientSet, nc, cl, podName)
						if err != nil {
							return err
						}
						pvcSet := make(map[types.UID]v1alpha1.EmptyStruct)
						for _, pvc := range pvcs {
							pvcSet[pvc.UID] = v1alpha1.EmptyStruct{}
						}
						fh.PVCSet = pvcSet
						nc.Status.Metad.FailureHosts[podName] = fh
						klog.Infof("metad pod [%s/%s] failover period exceeds %s", nc.Namespace, podName, nc.Spec.FailoverPeriod.Duration.String())
					}
				}
				continue
			}
			failureHost := v1alpha1.FailureHost{
				Host:         host.HostAddr.Host,
				CreationTime: metav1.Time{Time: time.Now()},
			}
			nc.Status.Metad.FailureHosts[podName] = failureHost
			klog.Infof("offline meta host %s found", host.HostAddr.Host)
		}
	}

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
	volumeStatus, err := syncPVC(nc.MetadComponent(), c.clientSet.StorageClass(), c.clientSet.PVC())
	if err != nil {
		return nil
	}
	nc.MetadComponent().SetVolumeStatus(volumeStatus)
	return nil
}

func (c *metadCluster) shouldRecover(nc *v1alpha1.NebulaCluster) (bool, []string, error) {
	if nc.Status.Metad.FailureHosts == nil {
		return true, nil, nil
	}

	m := make(map[string]string)
	for podName, fh := range nc.Status.Metad.FailureHosts {
		pod, err := c.clientSet.Pod().GetPod(nc.Namespace, podName)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, nil, err
		}
		if pod == nil {
			continue
		}
		if isPodHealthy(pod) {
			m[fh.Host] = podName
		}
	}
	if len(m) == 0 {
		return false, nil, nil
	}

	options, err := nebula.ClientOptions(nc, nebula.SetIsMeta(true))
	if err != nil {
		return false, nil, err
	}
	hosts := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(hosts, options...)
	if err != nil {
		return false, nil, err
	}
	defer func() {
		_ = metaClient.Disconnect()
	}()

	onlineHosts := make([]string, 0)
	hostItems, err := metaClient.ListHosts(meta.ListHostType_META)
	if err != nil {
		return false, nil, err
	}
	thriftPort := nc.MetadComponent().GetPort(v1alpha1.MetadPortNameThrift)
	for _, host := range hostItems {
		podName, ok := m[host.HostAddr.Host]
		if ok && host.Status == meta.HostStatus_ONLINE && host.HostAddr.Port == thriftPort {
			onlineHosts = append(onlineHosts, podName)
		}
	}
	r := len(onlineHosts) > 0
	return r, onlineHosts, nil
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

func (f *FakeMetadCluster) Delete(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
