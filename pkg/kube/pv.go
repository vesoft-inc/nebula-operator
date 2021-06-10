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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

type PersistentVolume interface {
	CreatePersistentVolume(pv *corev1.PersistentVolume) error
	GetPersistentVolume(name string) (*corev1.PersistentVolume, error)
	PatchPVReclaimPolicy(pv *corev1.PersistentVolume, policy corev1.PersistentVolumeReclaimPolicy) error
	UpdateMetaInfo(obj runtime.Object, pv *corev1.PersistentVolume) error
	UpdatePersistentVolume(pv *corev1.PersistentVolume) error
}

type pvClient struct {
	kubecli client.Client
}

func NewPV(kubecli client.Client) PersistentVolume {
	return &pvClient{kubecli: kubecli}
}

func (p *pvClient) CreatePersistentVolume(pv *corev1.PersistentVolume) error {
	log := getLog().WithValues("namespace", pv.Namespace, "name", pv.Name)
	if err := p.kubecli.Create(context.TODO(), pv); err != nil {
		return err
	}
	log.Info("namespace created")
	return nil
}

func (p *pvClient) GetPersistentVolume(name string) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	err := p.kubecli.Get(context.TODO(), types.NamespacedName{
		Name: name,
	}, pv)
	if err != nil {
		return nil, err
	}
	return pv, nil
}

func (p *pvClient) PatchPVReclaimPolicy(pv *corev1.PersistentVolume, reclaimPolicy corev1.PersistentVolumeReclaimPolicy) error {
	log := getLog().WithValues("namespace", pv.Namespace, "name", pv.Name)
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"persistentVolumeReclaimPolicy":"%s"}}`, reclaimPolicy))
	patch := client.RawPatch(types.StrategicMergePatchType, patchBytes)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return p.kubecli.Patch(context.TODO(), pv, patch)
	})
	if err != nil {
		return err
	}
	log.Info("namespace patched reclaim policy")
	return nil
}

func (p *pvClient) UpdateMetaInfo(obj runtime.Object, pv *corev1.PersistentVolume) error {
	log := getLog().WithValues("name", pv.GetName())
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return fmt.Errorf("%+v is not a runtime.Object", obj)
	}
	namespace := metaObj.GetNamespace()
	log = log.WithValues("namespace", namespace)
	if pv.Annotations == nil {
		pv.Annotations = make(map[string]string)
	}
	if pv.Labels == nil {
		pv.Labels = make(map[string]string)
	}
	pvcRef := pv.Spec.ClaimRef
	if pvcRef == nil {
		log.Info("pv doesn't have a ClaimRef, skipping")
		return nil
	}

	pvcName := pvcRef.Name
	pvc := &corev1.PersistentVolumeClaim{}
	err := p.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      pvcName,
	}, pvc)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Info("pv: pvc doesn't exist, skipping", "pvcName", pvcName)
		return nil
	}
	componentType := pvc.Labels[label.ComponentLabelKey]
	podName := pvc.Annotations[annotation.AnnPodNameKey]
	pv.Labels[label.ComponentLabelKey] = componentType
	pv.Labels[label.ClusterLabelKey] = pvc.Labels[label.ClusterLabelKey]
	pv.Labels[label.NameLabelKey] = pvc.Labels[label.NameLabelKey]
	pv.Labels[label.ManagedByLabelKey] = pvc.Labels[label.ManagedByLabelKey]
	pv.Annotations[annotation.AnnPodNameKey] = podName

	return p.UpdatePersistentVolume(pv)
}

func (p *pvClient) UpdatePersistentVolume(pv *corev1.PersistentVolume) error {
	log := getLog().WithValues("name", pv.GetName())
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return p.kubecli.Update(context.TODO(), pv)
	})
	if err != nil {
		return err
	}
	log.V(4).Info("pv updated")
	return nil
}
