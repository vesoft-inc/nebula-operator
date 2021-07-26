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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Pod interface {
	GetPod(namespace string, name string) (*corev1.Pod, error)
	UpdatePod(pod *corev1.Pod) error
	DeletePod(namespace string, name string) error
	ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error)
}

type podClient struct {
	kubecli client.Client
}

func NewPod(kubecli client.Client) Pod {
	return &podClient{kubecli: kubecli}
}

func (pd *podClient) GetPod(namespace, name string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := pd.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func (pd *podClient) UpdatePod(pod *corev1.Pod) error {
	log := getLog().WithValues("namespace", pod.GetNamespace(), "name", pod.GetName())
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return pd.kubecli.Update(context.TODO(), pod)
	})
	if err != nil {
		return err
	}
	log.Info("pod updated")
	return nil
}

func (pd *podClient) DeletePod(namespace, name string) error {
	log := getLog().WithValues("namespace", namespace, "name", name)
	pod := &corev1.Pod{}
	if err := pd.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, pod); err != nil {
		return err
	}
	log.Info("pod deleted")
	return pd.kubecli.Delete(context.TODO(), pod)
}

func (pd *podClient) ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := pd.kubecli.List(context.TODO(), podList, &client.ListOptions{LabelSelector: selector, Namespace: namespace}); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
