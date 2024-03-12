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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	podutils "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/async"
	"github.com/vesoft-inc/nebula-operator/pkg/util/codec"
	"github.com/vesoft-inc/nebula-operator/pkg/util/config"
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/hash"
	httputil "github.com/vesoft-inc/nebula-operator/pkg/util/http"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

var suspendOrder = []v1alpha1.ComponentType{
	v1alpha1.GraphdComponentType,
	v1alpha1.StoragedComponentType,
	v1alpha1.MetadComponentType,
}

const (
	InPlaceGracePeriodSeconds = 60

	SyncVolumeConcurrency = 5
)

func syncComponentStatus(
	component v1alpha1.NebulaClusterComponent,
	status *v1alpha1.ComponentStatus,
	workload *unstructured.Unstructured,
) error {
	if workload == nil {
		return nil
	}

	err := setWorkloadStatus(workload, status)
	if err != nil {
		return err
	}

	image := getContainerImage(workload, component.ComponentType().String())
	if image != "" && strings.Contains(image, ":") {
		status.Version = strings.Split(image, ":")[1]
	}

	component.UpdateComponentStatus(status)

	return nil
}

func setWorkloadStatus(obj *unstructured.Unstructured, status *v1alpha1.ComponentStatus) error {
	workload := &v1alpha1.WorkloadStatus{}
	data, err := json.Marshal(extender.GetStatus(obj))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &workload); err != nil {
		return err
	}
	status.Workload = workload
	return nil
}

func syncService(newSvc *corev1.Service, svcClient kube.Service) error {
	oldSvcTmp, err := svcClient.GetService(newSvc.Namespace, newSvc.Name)
	if apierrors.IsNotFound(err) {
		if err := setServiceLastAppliedConfigAnnotation(newSvc); err != nil {
			return err
		}
		return svcClient.CreateService(newSvc)
	}
	if err != nil {
		return err
	}

	oldSvc := oldSvcTmp.DeepCopy()
	equal, err := serviceEqual(newSvc, oldSvc)
	if err != nil {
		return err
	}

	annoEqual := maputil.IsSubMap(newSvc.Annotations, oldSvc.Annotations)
	isOrphan := metav1.GetControllerOf(oldSvc) == nil

	if !equal || !annoEqual || isOrphan {
		svc := *oldSvc
		svc.Spec = newSvc.Spec
		if err := setServiceLastAppliedConfigAnnotation(&svc); err != nil {
			return err
		}
		if oldSvc.Spec.ClusterIP != "" {
			svc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
		}
		for k, v := range newSvc.Annotations {
			svc.Annotations[k] = v
		}
		if isOrphan {
			svc.OwnerReferences = newSvc.OwnerReferences
			svc.Labels = newSvc.Labels
		}
		if err := svcClient.UpdateService(&svc); err != nil {
			return err
		}
	}

	return nil
}

func setServiceLastAppliedConfigAnnotation(svc *corev1.Service) error {
	b, err := json.Marshal(svc.Spec)
	if err != nil {
		return err
	}
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	svc.Annotations[annotation.AnnLastAppliedConfigKey] = string(b)
	return nil
}

func serviceEqual(newSvc, oldSvc *corev1.Service) (bool, error) {
	oldSpec := corev1.ServiceSpec{}
	if lastAppliedConfig, ok := oldSvc.Annotations[annotation.AnnLastAppliedConfigKey]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec)
		if err != nil {
			return false, err
		}
		return apiequality.Semantic.DeepEqual(oldSpec, newSvc.Spec), nil
	}
	return false, nil
}

func copyZoneData(component v1alpha1.NebulaClusterComponent, cm *corev1.ConfigMap) *corev1.ConfigMap {
	updated := generateZoneConfigMap(component)
	updated.Data = cm.Data
	return updated
}

func generateZoneConfigMap(component v1alpha1.NebulaClusterComponent) *corev1.ConfigMap {
	namespace := component.GetNamespace()
	labels := component.GenerateLabels()
	cmName := fmt.Sprintf("%s-%s", component.GetName(), v1alpha1.ZoneSuffix)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cmName,
			Namespace:       namespace,
			OwnerReferences: component.GenerateOwnerReferences(),
			Labels:          labels,
		},
	}
	cm.Data = map[string]string{component.GetName(): ""}
	return cm
}

func syncZoneConfigMap(component v1alpha1.NebulaClusterComponent, cmClient kube.ConfigMap) error {
	cm := generateZoneConfigMap(component)
	return cmClient.CreateOrUpdateConfigMap(cm)
}

func syncConfigMap(
	component v1alpha1.NebulaClusterComponent,
	cmClient kube.ConfigMap,
	template,
	cmKey string,
) (*corev1.ConfigMap, string, error) {
	cmHash := hash.Hash(template)
	cm := component.GenerateConfigMap()
	cfg := component.GetConfig()
	if cfg != nil {
		namespace := component.GetNamespace()
		clusterName := component.GetClusterName()
		flags := staticOrStartupFlags(cfg)
		klog.V(3).Infof("cluster [%s/%s] sync %s configmap with custom static or startup configs %v", namespace, clusterName,
			component.ComponentType().String(), flags)
		customConf := config.AppendCustomConfig(template, flags)
		cm.Data[cmKey] = customConf
		cmHash = hash.Hash(customConf)
	}

	if err := cmClient.CreateOrUpdateConfigMap(cm); err != nil {
		return nil, "", err
	}
	return cm, cmHash, nil
}

func staticOrStartupFlags(config map[string]string) map[string]string {
	static := make(map[string]string)
	for k, v := range config {
		if _, ok := v1alpha1.DynamicFlags[k]; !ok {
			static[k] = v
		}
	}
	return static
}

func updateDynamicFlags(endpoints []string, oldAnnotations, newAnnotations map[string]string) error {
	newFlags := make(map[string]string)
	newFlagsVal, ok := newAnnotations[annotation.AnnLastAppliedDynamicFlagsKey]
	if ok {
		if err := json.Unmarshal([]byte(newFlagsVal), &newFlags); err != nil {
			return err
		}
	}
	oldFlags := make(map[string]string)
	oldFlagsVal, ok := oldAnnotations[annotation.AnnLastAppliedDynamicFlagsKey]
	if ok {
		if err := json.Unmarshal([]byte(oldFlagsVal), &oldFlags); err != nil {
			return err
		}
	}
	if len(newFlags) == 0 && len(oldFlags) == 0 {
		return nil
	}
	_, removed := maputil.IntersectionDifference(oldFlags, newFlags)
	if len(removed) > 0 {
		maputil.ResetMap(removed, v1alpha1.DynamicFlags)
		newFlags = maputil.MergeStringMaps(true, newFlags, removed)
	}
	klog.V(1).Infof("dynamic flags: %v", newFlags)
	str, err := codec.Encode(newFlags)
	if err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		url := fmt.Sprintf("http://%s/flags", endpoint)
		if _, err := httputil.PutRequest(url, []byte(str)); err != nil {
			return err
		}
	}
	return nil
}

func getContainerImage(obj *unstructured.Unstructured, containerName string) string {
	if obj == nil {
		return ""
	}
	containers := extender.GetContainers(obj)
	for _, ctr := range containers {
		if ctr["name"] == containerName {
			return ctr["image"].(string)
		}
	}
	return ""
}

func isUpdating(component v1alpha1.NebulaClusterComponent, podClient kube.Pod, obj *unstructured.Unstructured) (bool, error) {
	if extender.IsUpdating(obj) {
		return true, nil
	}

	selector, err := label.Label(component.GenerateLabels()).Selector()
	if err != nil {
		return false, err
	}

	pods, err := podClient.ListPods(component.GetNamespace(), selector)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get pods for cluster [%s/%s], selector %s, error: %s",
			component.GetNamespace(),
			component.GetClusterName(),
			selector,
			err,
		)
	}
	for i := range pods {
		pod := pods[i]
		revisionHash, exist := pod.Labels[appsv1.ControllerRevisionHashLabelKey]
		if !exist {
			return false, nil
		}
		if component.GetUpdateRevision() != "" &&
			revisionHash != component.GetUpdateRevision() {
			return true, nil
		}
	}
	return false, nil
}

func setPartition(obj *unstructured.Unstructured, upgradeOrdinal int64, advanced bool) error {
	return extender.SetUpdatePartition(obj, upgradeOrdinal, InPlaceGracePeriodSeconds, advanced)
}

func getNextUpdatePod(component v1alpha1.NebulaClusterComponent, replicas int32, podClient kube.Pod) (int32, error) {
	namespace := component.GetNamespace()
	updateRevision := component.GetUpdateRevision()
	for index := replicas - 1; index >= 0; index-- {
		podName := component.GetPodName(index)
		pod, err := podClient.GetPod(namespace, podName)
		if err != nil {
			return -1, err
		}
		revision, exist := pod.Labels[appsv1.ControllerRevisionHashLabelKey]
		if !exist {
			return -1, &errors.ReconcileError{Msg: fmt.Sprintf("rolling updated pod %s has no label: %s",
				podName, appsv1.ControllerRevisionHashLabelKey)}
		}
		if revision == updateRevision {
			if pod.Status.Phase != corev1.PodRunning {
				return -1, &errors.ReconcileError{Msg: fmt.Sprintf("rolling updated pod %s is not running", podName)}
			}
			continue
		}

		return index, nil
	}
	return -1, nil
}

func setLastConfig(actual, desired *unstructured.Unstructured) error {
	spec := make(map[string]interface{})
	if lastAppliedConfig, ok := actual.GetAnnotations()[annotation.AnnLastAppliedConfigKey]; ok {
		if err := json.Unmarshal([]byte(lastAppliedConfig), &spec); err != nil {
			return err
		}
	}

	return extender.SetSpecField(desired, spec["template"], "template")
}

func contains(ss []int32, lookingFor int32) bool {
	for _, s := range ss {
		if lookingFor == s {
			return true
		}
	}

	return false
}

func setDeploymentLastAppliedConfigAnnotation(deploy *appsv1.Deployment) error {
	b, err := json.Marshal(deploy.Spec)
	if err != nil {
		return err
	}
	if deploy.Annotations == nil {
		deploy.Annotations = map[string]string{}
	}
	deploy.Annotations[annotation.AnnLastAppliedConfigKey] = string(b)
	deploy.Annotations[annotation.AnnLastSyncTimestampKey] = time.Now().Format(time.RFC3339)
	return nil
}

func updateDeployment(clientSet kube.ClientSet, newDeploy, oldDeploy *appsv1.Deployment) error {
	isOrphan := metav1.GetControllerOf(oldDeploy) == nil
	if newDeploy.Annotations == nil {
		newDeploy.Annotations = map[string]string{}
	}
	if oldDeploy.Annotations == nil {
		oldDeploy.Annotations = map[string]string{}
	}

	if deploymentEqual(newDeploy, oldDeploy) && !isOrphan {
		return nil
	}

	deploy := oldDeploy
	deploy.Labels = newDeploy.Labels
	deploy.Annotations = newDeploy.Annotations
	deploy.Spec.Replicas = newDeploy.Spec.Replicas
	deploy.Spec.Template = newDeploy.Spec.Template
	deploy.Spec.Strategy = newDeploy.Spec.Strategy
	if isOrphan {
		deploy.OwnerReferences = newDeploy.OwnerReferences
	}

	var podConfig string
	var exists bool
	if oldDeploy.Spec.Template.Annotations != nil {
		podConfig, exists = oldDeploy.Spec.Template.Annotations[annotation.AnnLastAppliedConfigKey]
	}
	if exists {
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = map[string]string{}
		}
		deploy.Spec.Template.Annotations[annotation.AnnLastAppliedConfigKey] = podConfig
	}
	v, ok := oldDeploy.Annotations[annotation.AnnLastSyncTimestampKey]
	if ok {
		deploy.Annotations[annotation.AnnLastSyncTimestampKey] = v
	}

	err := setDeploymentLastAppliedConfigAnnotation(deploy)
	if err != nil {
		return err
	}

	return clientSet.Deployment().UpdateDeployment(deploy)
}

func deploymentEqual(newDeploy, oldDeploy *appsv1.Deployment) bool {
	tmpAnno := map[string]string{}
	for k, v := range oldDeploy.Annotations {
		if k != annotation.AnnLastAppliedConfigKey && k != annotation.AnnLastSyncTimestampKey &&
			k != annotation.AnnDeploymentRevision {
			tmpAnno[k] = v
		}
	}

	if !apiequality.Semantic.DeepEqual(newDeploy.Annotations, tmpAnno) {
		return false
	}
	oldConfig := appsv1.DeploymentSpec{}
	if lastAppliedConfig, ok := oldDeploy.Annotations[annotation.AnnLastAppliedConfigKey]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldConfig)
		if err != nil {
			return false
		}
		tmpTemplate := oldConfig.Template.DeepCopy()
		delete(tmpTemplate.Annotations, annotation.AnnLastAppliedConfigKey)
		return apiequality.Semantic.DeepEqual(oldConfig.Replicas, newDeploy.Spec.Replicas) &&
			apiequality.Semantic.DeepEqual(*tmpTemplate, newDeploy.Spec.Template) &&
			apiequality.Semantic.DeepEqual(oldConfig.Strategy, newDeploy.Spec.Strategy)
	}
	return false
}

func setPodLastAppliedConfigAnnotation(pod *corev1.Pod) error {
	b, err := json.Marshal(pod.Spec)
	if err != nil {
		return err
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[annotation.AnnLastAppliedConfigKey] = string(b)
	pod.Annotations[annotation.AnnLastSyncTimestampKey] = time.Now().Format(time.RFC3339)
	return nil
}

func podEqual(newPod, oldPod *corev1.Pod) bool {
	tmpAnno := map[string]string{}
	for k, v := range oldPod.Annotations {
		if k != annotation.AnnLastAppliedConfigKey && k != annotation.AnnLastSyncTimestampKey {
			tmpAnno[k] = v
		}
	}

	if !apiequality.Semantic.DeepEqual(newPod.Annotations, tmpAnno) {
		return false
	}
	oldConfig := corev1.PodSpec{}
	if lastAppliedConfig, ok := oldPod.Annotations[annotation.AnnLastAppliedConfigKey]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldConfig)
		if err != nil {
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig, newPod.Spec)
	}
	return false
}

func updateSinglePod(clientSet kube.ClientSet, newPod, oldPod *corev1.Pod) error {
	isOrphan := metav1.GetControllerOf(oldPod) == nil
	if podEqual(newPod, oldPod) && !isOrphan {
		return nil
	}

	if err := setPodLastAppliedConfigAnnotation(newPod); err != nil {
		return err
	}

	if err := clientSet.Pod().DeletePod(oldPod.Namespace, oldPod.Name, false); err != nil {
		return err
	}

	return clientSet.Pod().CreatePod(newPod)
}

func syncPVC(component v1alpha1.NebulaClusterComponent, scClient kube.StorageClass, pvcClient kube.PersistentVolumeClaim) (*v1alpha1.VolumeStatus, error) {
	replicas := int(component.ComponentSpec().Replicas())
	volumeClaims, err := component.GenerateVolumeClaim()
	if err != nil {
		return nil, err
	}
	if len(volumeClaims) == 0 {
		return nil, nil
	}

	volumeProvisioned := make(map[string]bool)
	volumes := make(map[string]v1alpha1.ProvisionedVolume)
	var lock sync.Mutex
	for i := range volumeClaims {
		volumeClaim := volumeClaims[i]
		group := async.NewGroup(context.TODO(), SyncVolumeConcurrency)
		for i := 0; i < replicas; i++ {
			pvcName := fmt.Sprintf("%s-%s-%d", volumeClaim.Name, component.GetName(), i)
			worker := func() error {
				oldPVC, err := pvcClient.GetPVC(component.GetNamespace(), pvcName)
				if err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
				}
				if oldPVC == nil {
					return nil
				}

				lock.Lock()
				volumes[pvcName] = v1alpha1.ProvisionedVolume{
					VolumeName:   oldPVC.Spec.VolumeName,
					StorageClass: pointer.StringDeref(oldPVC.Spec.StorageClassName, "default"),
					Capacity:     oldPVC.Status.Capacity.Storage().String(),
				}
				volumeProvisioned[pvcName] = false
				lock.Unlock()

				cmpVal := volumeClaim.Spec.Resources.Requests.Storage().Cmp(*oldPVC.Spec.Resources.Requests.Storage())
				if cmpVal == 0 {
					lock.Lock()
					volumeProvisioned[pvcName] = true
					lock.Unlock()
					return nil
				}

				if cmpVal < 0 {
					klog.Warningf("skip to resize PVC %s: storage request cannot be shrunk to %s",
						pvcName, volumeClaim.Spec.Resources.Requests.Storage().String())
					return nil
				}

				var sc *storagev1.StorageClass
				scName := pointer.StringDeref(volumeClaim.Spec.StorageClassName, "")
				if scName != "" {
					sc, err = scClient.GetStorageClass(scName)
					if err != nil {
						return err
					}
				} else {
					sc, err = scClient.GetDefaultStorageClass()
					if err != nil {
						return err
					}
				}
				if !isVolumeExpansionSupported(sc) {
					klog.V(4).Infof("storageclass %s does not allow volume expansion", scName)
					return nil
				}

				if cmpVal > 0 {
					klog.Infof("expand PVC %s size to %s", pvcName, volumeClaim.Spec.Resources.Requests.Storage().String())
					// only update storage
					oldPVC.Spec.Resources.Requests = volumeClaim.Spec.Resources.Requests
					if err = pvcClient.UpdatePVC(oldPVC); err != nil {
						return err
					}
				}
				return nil
			}

			group.Add(func(stopCh chan interface{}) {
				stopCh <- worker()
			})
		}

		if err := group.Wait(); err != nil {
			return nil, err
		}
	}

	volumeStatus := &v1alpha1.VolumeStatus{
		ProvisionedVolumes: volumes,
		ProvisionedDone:    true,
	}
	for _, p := range volumeProvisioned {
		if !p {
			volumeStatus.ProvisionedDone = false
			break
		}
	}

	return volumeStatus, nil
}

func isVolumeExpansionSupported(sc *storagev1.StorageClass) bool {
	if sc.AllowVolumeExpansion == nil {
		return false
	}
	return *sc.AllowVolumeExpansion
}

func suspendComponent(
	workloadClient kube.Workload,
	component v1alpha1.NebulaClusterComponent,
	workload *unstructured.Unstructured) (bool, error) {
	nc := component.GetNebulaCluster()
	suspending := component.IsSuspending()
	if !nc.IsSuspendEnabled() {
		if suspending {
			component.SetPhase(v1alpha1.RunningPhase)
			return true, nil
		}
		klog.V(4).Infof("component %s is not needed to be suspended", component.GetName())
		return false, nil
	}
	if !suspending {
		if s, reason := canSuspendComponent(component); !s {
			klog.Warningf("component %s can not be suspended: %s", component.GetName(), reason)
			return false, nil
		}
		component.SetPhase(v1alpha1.SuspendPhase)
		return true, nil
	}
	if workload != nil {
		if err := workloadClient.DeleteWorkload(workload); err != nil {
			return false, err
		}
	}
	component.SetWorkloadStatus(nil)
	return true, nil
}

func canSuspendComponent(component v1alpha1.NebulaClusterComponent) (bool, string) {
	nc := component.GetNebulaCluster()
	if component.GetPhase() != v1alpha1.RunningPhase && component.GetPhase() != v1alpha1.SuspendPhase {
		return false, "component phase is not Running or Suspend"
	}
	for _, ct := range suspendOrder {
		if ct == component.ComponentType() {
			break
		}
		c, err := nc.ComponentByType(ct)
		if err != nil {
			return false, err.Error()
		}
		if !c.IsSuspended() {
			return false, fmt.Sprintf("waiting for component %s to be suspended", ct)
		}
	}
	return true, ""
}

func isPodPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

func isPodHealthy(pod *corev1.Pod) bool {
	return isPodRunningAndReady(pod) && !isPodTerminating(pod)
}

func isPodRunningAndReady(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && podutils.IsPodReady(pod)
}

func isPodTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func isPodConditionScheduledTrue(conditions []corev1.PodCondition) bool {
	podSchCond := getPodConditionFromList(conditions, corev1.PodScheduled)
	return podSchCond != nil && podSchCond.Status == corev1.ConditionTrue
}

func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) *corev1.PodCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
