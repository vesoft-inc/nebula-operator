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

	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
)

type PersistentVolumeClaim interface {
	CreatePVC(pvc *corev1.PersistentVolumeClaim) error
	GetPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error)
	UpdateMetaInfo(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod) error
	UpdatePVC(pvc *corev1.PersistentVolumeClaim) error
	DeletePVC(namespace string, name string) error
	ListPVCs(namespace string, selector labels.Selector) ([]corev1.PersistentVolumeClaim, error)
}

type pvcClient struct {
	kubecli client.Client
}

func NewPVC(kubecli client.Client) PersistentVolumeClaim {
	return &pvcClient{kubecli: kubecli}
}

func (p *pvcClient) CreatePVC(pvc *corev1.PersistentVolumeClaim) error {
	log := getLog().WithValues("namespace", pvc.GetNamespace(), "name", pvc.GetName())
	if err := p.kubecli.Create(context.TODO(), pvc); err != nil {
		return err
	}
	log.Info("pvc created ")
	return nil
}

func (p *pvcClient) GetPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := p.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, pvc)
	if err != nil {
		return nil, err
	}
	return pvc, nil
}

func (p *pvcClient) ListPVCs(namespace string, selector labels.Selector) ([]corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := p.kubecli.List(context.TODO(), pvcList, &client.ListOptions{LabelSelector: selector, Namespace: namespace}); err != nil {
		return nil, err
	}
	return pvcList.Items, nil
}

func (p *pvcClient) UpdateMetaInfo(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod) error {
	podName := pod.GetName()
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Labels[annotation.AnnPodNameKey] = podName
	pvc.Annotations[annotation.AnnPodNameKey] = podName

	return p.UpdatePVC(pvc)
}

func (p *pvcClient) UpdatePVC(pvc *corev1.PersistentVolumeClaim) error {
	log := getLog().WithValues("namespace", pvc.GetNamespace(), "name", pvc.GetName())
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return p.kubecli.Update(context.TODO(), pvc)
	})
	if err != nil {
		return err
	}
	log.V(4).Info("pvc updated")
	return nil
}

func (p *pvcClient) DeletePVC(namespace, name string) error {
	log := getLog().WithValues("namespace", namespace, "name", name)
	pvc, err := p.GetPVC(namespace, name)
	if err != nil {
		return err
	}
	if err := p.kubecli.Delete(context.TODO(), pvc); err != nil {
		return err
	}
	log.Info("pvc deleted")
	return nil
}
