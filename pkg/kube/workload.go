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
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		return nil, err
	}
	return workload, nil
}

func (w *workloadClient) CreateWorkload(obj *unstructured.Unstructured) error {
	log := getLog().WithValues("kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
	if err := w.kubecli.Create(context.TODO(), obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Error(err, "workload already exists")
			return nil
		}
		return err
	}
	log.Info("workload created")
	return nil
}

func (w *workloadClient) UpdateWorkload(obj *unstructured.Unstructured) error {
	log := getLog().WithValues("kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return w.kubecli.Update(context.TODO(), obj)
	})
	if err != nil {
		return fmt.Errorf("workload kind %s %s/%s update failed: %v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
	}
	log.Info("workload updated")
	return nil
}
