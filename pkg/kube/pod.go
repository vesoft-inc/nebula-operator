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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Pod interface {
	GetPod(namespace string, name string) (*corev1.Pod, error)
	CreatePod(pod *corev1.Pod) error
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
		klog.V(4).ErrorS(err, "failed to get pod", "namespace", namespace, "name", name)
		return nil, err
	}
	return pod, nil
}

func (pd *podClient) CreatePod(pod *corev1.Pod) error {
	if err := pd.kubecli.Create(context.TODO(), pod); err != nil {
		if apierrors.IsAlreadyExists(err) && !strings.Contains(err.Error(), "being deleted") {
			return nil
		}
		return err
	}
	return nil
}

func (pd *podClient) UpdatePod(pod *corev1.Pod) error {
	ns := pod.GetNamespace()
	podName := pod.GetName()
	podLabels := pod.GetLabels()
	annotations := pod.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := pd.GetPod(ns, podName); err == nil {
			pod = updated.DeepCopy()
			pod.SetLabels(podLabels)
			pod.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get pod [%s/%s] failed: %v", ns, podName, err))
			return err
		}

		updateErr := pd.kubecli.Update(context.TODO(), pod)
		if updateErr == nil {
			klog.Infof("pod [%s/%s] updated successfully", ns, podName)
			return nil
		}
		return updateErr
	})
}

func (pd *podClient) DeletePod(namespace, name string) error {
	pod := &corev1.Pod{}
	if err := pd.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, pod); err != nil {
		return err
	}

	policy := metav1.DeletePropagationBackground
	options := &client.DeleteOptions{
		PropagationPolicy: &policy,
	}
	return pd.kubecli.Delete(context.TODO(), pod, options)
}

func (pd *podClient) ListPods(namespace string, selector labels.Selector) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := pd.kubecli.List(context.TODO(), podList, &client.ListOptions{LabelSelector: selector, Namespace: namespace}); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
