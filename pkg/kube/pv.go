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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
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
	return p.kubecli.Create(context.TODO(), pv)
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
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"persistentVolumeReclaimPolicy":"%s"}}`, reclaimPolicy))
	patch := client.RawPatch(types.StrategicMergePatchType, patchBytes)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return p.kubecli.Patch(context.TODO(), pv, patch)
	})
	if err != nil {
		return err
	}
	klog.Infof("PV %s patched reclaim policy", pv.GetName())
	return nil
}

func (p *pvClient) UpdateMetaInfo(obj runtime.Object, pv *corev1.PersistentVolume) error {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return fmt.Errorf("%+v is not a runtime.Object", obj)
	}
	namespace := metaObj.GetNamespace()
	if pv.Annotations == nil {
		pv.Annotations = make(map[string]string)
	}
	if pv.Labels == nil {
		pv.Labels = make(map[string]string)
	}
	pvcRef := pv.Spec.ClaimRef
	if pvcRef == nil {
		klog.Infof("PV %s doesn't have a claimRef, skipping", pv.GetName())
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
		klog.Infof("PV: PVC %s doesn't exist, skipping", pvcName)
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
	pvName := pv.GetName()
	labels := pv.GetLabels()
	annotations := pv.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := p.GetPersistentVolume(pvName); err == nil {
			pv = updated.DeepCopy()
			pv.SetLabels(labels)
			pv.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get PV %s failed: %v", pvName, err))
			return err
		}

		updateErr := p.kubecli.Update(context.TODO(), pv)
		if updateErr == nil {
			klog.V(4).Infof("PV %s updated successfully", pvName)
			return nil
		}
		return updateErr
	})
}
