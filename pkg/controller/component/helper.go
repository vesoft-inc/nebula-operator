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
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/util/config"
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/hash"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

const (
	InPlaceGracePeriodSeconds = 60
)

func syncComponentStatus(
	component v1alpha1.NebulaClusterComponentter,
	status *v1alpha1.ComponentStatus,
	workload *unstructured.Unstructured) error {
	if workload == nil {
		return nil
	}

	err := setWorkloadStatus(workload, status)
	if err != nil {
		return err
	}

	image := getContainerImage(workload, component.Type().String())
	if image != "" && strings.Contains(image, ":") {
		status.Version = strings.Split(image, ":")[1]
	}

	component.UpdateComponentStatus(status)

	return nil
}

func setWorkloadStatus(obj *unstructured.Unstructured, status *v1alpha1.ComponentStatus) error {
	workload := v1alpha1.WorkloadStatus{}
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

func syncService(component v1alpha1.NebulaClusterComponentter, svcClient kube.Service) error {
	newSvc := component.GenerateService()
	if newSvc == nil {
		return nil
	}

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

func syncConfigMap(
	component v1alpha1.NebulaClusterComponentter,
	cmClient kube.ConfigMap,
	template,
	fileName string) (*corev1.ConfigMap, string, error) {
	cmHash := hash.Hash(template)
	cm := component.GenerateConfigMap()
	if component.GetConfig() != nil {
		customConf := config.AppendCustomConfig(template, component.GetConfig())
		cm.Data[fileName] = customConf
		cmHash = hash.Hash(customConf)
	}

	if err := cmClient.CreateOrUpdateConfigMap(cm); err != nil {
		return nil, "", err
	}
	return cm, cmHash, nil
}

func getContainerImage(
	obj *unstructured.Unstructured,
	containerName string) string {
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

func isUpdating(
	component v1alpha1.NebulaClusterComponentter,
	podClient kube.Pod,
	obj *unstructured.Unstructured) (bool, error) {
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
			"failed to get pods for cluster %s/%s, selector %s, error: %s",
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

func getNextUpdatePod(component v1alpha1.NebulaClusterComponentter, replicas int32, podClient kube.Pod) (int32, error) {
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
