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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/config"
	controllerutil "github.com/vesoft-inc/nebula-operator/pkg/util/controller"
	"github.com/vesoft-inc/nebula-operator/pkg/util/hash"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

func syncComponentStatus(
	component v1alpha1.NebulaClusterComponentter,
	extender controllerutil.UnstructuredExtender,
	status *v1alpha1.ComponentStatus,
	workload *unstructured.Unstructured) error {
	if workload == nil {
		return nil
	}

	err := setWorkloadStatus(extender, workload, status)
	if err != nil {
		return err
	}

	image := controllerutil.GetContainerImage(extender, workload, component.Type().String())
	if image != "" && strings.Contains(image, ":") {
		status.Version = strings.Split(image, ":")[1]
	}

	component.UpdateComponentStatus(status)

	return nil
}

func setWorkloadStatus(
	extender controllerutil.UnstructuredExtender,
	obj *unstructured.Unstructured,
	status *v1alpha1.ComponentStatus,
) error {
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
		if err := controllerutil.SetServiceLastAppliedConfigAnnotation(newSvc); err != nil {
			return err
		}
		return svcClient.CreateService(newSvc)
	}
	if err != nil {
		klog.Errorf("failed to get svc %s for cluster %s/%s, error: %s", newSvc.Name, newSvc.Namespace, component.GetClusterName(), err)
		return err
	}

	oldSvc := oldSvcTmp.DeepCopy()
	equal, err := controllerutil.ServiceEqual(newSvc, oldSvc)
	if err != nil {
		return err
	}

	annoEqual := maputil.IsSubMap(newSvc.Annotations, oldSvc.Annotations)
	isOrphan := metav1.GetControllerOf(oldSvc) == nil

	if !equal || !annoEqual || isOrphan {
		svc := *oldSvc
		svc.Spec = newSvc.Spec
		if err := controllerutil.SetServiceLastAppliedConfigAnnotation(&svc); err != nil {
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
