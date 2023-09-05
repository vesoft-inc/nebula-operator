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

package kube

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type Workload interface {
	GetWorkload(namespace string, name string, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error)
	CreateWorkload(obj *unstructured.Unstructured) error
	UpdateWorkload(obj *unstructured.Unstructured) error
}

type workloadClient struct {
	kubecli client.Client
}

func NewWorkload(kubecli client.Client) Workload {
	return &workloadClient{kubecli: kubecli}
}

func (w *workloadClient) GetWorkload(namespace, name string, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion(gvk.GroupVersion().String())
	workload.SetKind(gvk.Kind)
	err := w.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, workload)
	if err != nil {
		klog.Errorf("get workload [%s/%s] failed: %v", namespace, name, err)
		return nil, err
	}
	return workload, nil
}

func (w *workloadClient) CreateWorkload(obj *unstructured.Unstructured) error {
	if err := w.kubecli.Create(context.TODO(), obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Error(err, "workload already exists")
			return nil
		}
		return err
	}
	klog.Infof("workload %s/%s created successfully", obj.GetNamespace(), obj.GetName())
	return nil
}

func (w *workloadClient) UpdateWorkload(obj *unstructured.Unstructured) error {
	kind := obj.GetKind()
	ns := obj.GetNamespace()
	objName := obj.GetName()
	spec := getSpec(obj)
	labels := obj.GetLabels()
	annotations := obj.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		updated, err := w.GetWorkload(ns, objName, resource.StatefulSetKind)
		if err == nil {
			obj = updated.DeepCopy()
			setSpec(obj, spec)
			obj.SetLabels(labels)
			obj.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get workload %s/%s failed: %v", ns, objName, err))
		}

		updated, err = w.GetWorkload(ns, objName, resource.AdvancedStatefulSetKind)
		if err == nil {
			obj = updated.DeepCopy()
			setSpec(obj, spec)
			obj.SetLabels(labels)
			obj.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get workload %s/%s failed: %v", ns, objName, err))
		}

		updateErr := w.kubecli.Update(context.TODO(), obj)
		if updateErr == nil {
			klog.Infof("workload %s %s/%s updated successfully", kind, ns, objName)
			return nil
		}
		return updateErr
	})
}

func getSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil
	}
	return spec
}

func setSpec(obj *unstructured.Unstructured, value interface{}) error {
	return unstructured.SetNestedField(obj.Object, value, "spec")
}
