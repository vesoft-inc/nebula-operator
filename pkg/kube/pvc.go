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
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
)

type PersistentVolumeClaim interface {
	CreatePVC(pvc *corev1.PersistentVolumeClaim) error
	GetPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error)
	UpdateMetaInfo(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, isReclaimEnabled bool) error
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
	return p.kubecli.Create(context.TODO(), pvc)
}

func (p *pvcClient) GetPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := p.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, pvc)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get PVC", "namespace", namespace, "name", name)
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

func (p *pvcClient) UpdateMetaInfo(pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod, isReclaimEnabled bool) error {
	podName := pod.GetName()
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels[annotation.AnnPodNameKey] = podName
	pvc.Annotations[annotation.AnnPodNameKey] = podName
	pvc.Annotations[annotation.AnnPvReclaimKey] = strconv.FormatBool(isReclaimEnabled)

	return p.UpdatePVC(pvc)
}

func (p *pvcClient) UpdatePVC(pvc *corev1.PersistentVolumeClaim) error {
	ns := pvc.GetNamespace()
	pvcName := pvc.GetName()
	pvcLabels := pvc.GetLabels()
	annotations := pvc.GetAnnotations()
	requests := pvc.Spec.Resources.Requests

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := p.GetPVC(ns, pvcName); err == nil {
			if reflect.DeepEqual(pvcLabels, updated.GetLabels()) &&
				reflect.DeepEqual(annotations, updated.GetAnnotations()) &&
				reflect.DeepEqual(requests, updated.Spec.Resources.Requests) {
				return nil
			}

			pvc = updated.DeepCopy()
			pvc.SetLabels(pvcLabels)
			pvc.SetAnnotations(annotations)
			pvc.Spec.Resources.Requests = requests
		} else {
			utilruntime.HandleError(fmt.Errorf("get PV [%s/%s] failed: %v", ns, pvcName, err))
			return err
		}

		updateErr := p.kubecli.Update(context.TODO(), pvc)
		if updateErr == nil {
			klog.V(4).Infof("PVC [%s/%s] updated successfully", ns, pvcName)
			return nil
		}
		return updateErr
	})
}

func (p *pvcClient) DeletePVC(namespace, name string) error {
	pvc, err := p.GetPVC(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := p.kubecli.Delete(context.TODO(), pvc); err != nil {
		return err
	}
	klog.Infof("PVC [%s/%s] deleted successfully", namespace, name)
	return nil
}
