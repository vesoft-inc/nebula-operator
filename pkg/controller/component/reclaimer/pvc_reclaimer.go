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

package reclaimer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

type PVCReclaimer interface {
	Reclaim(cluster *v1alpha1.NebulaCluster) error
}

type pvcReclaimer struct {
	clientSet kube.ClientSet
}

func NewPVCReclaimer(clientSet kube.ClientSet) PVCReclaimer {
	return &pvcReclaimer{clientSet: clientSet}
}

func (p *pvcReclaimer) Reclaim(nc *v1alpha1.NebulaCluster) error {
	return p.reclaimPV(nc)
}

func (p *pvcReclaimer) reclaimPV(nc *v1alpha1.NebulaCluster) error {
	namespace := nc.GetNamespace()
	ncName := nc.GetName()

	pvcs, err := p.listPVCs(nc)
	if err != nil {
		return err
	}

	for i := range pvcs {
		pvc := pvcs[i]
		pvcName := pvc.GetName()
		if !label.Label(pvc.Labels).IsNebulaComponent() {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause component type is not graphd, metad or storaged", pvcName, namespace, ncName)
			continue
		}

		if pvc.Status.Phase != corev1.ClaimBound {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause pvc status is not bound", pvcName, namespace, ncName)
			continue
		}

		if pvc.DeletionTimestamp != nil {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause pvc has been deleted", pvcName, namespace, ncName)
			continue
		}

		if pvc.Annotations[annotation.AnnPVCDeferDeletingKey] == "" {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause pvc has not been marked as defer deleting pvc", pvcName, namespace, ncName)
			continue
		}

		podName, exist := pvc.Annotations[annotation.AnnPodNameKey]
		if !exist {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause pvc has no pod name annotation", pvcName, namespace, ncName)
			continue
		}

		_, err := p.clientSet.Pod().GetPod(namespace, podName)
		if err == nil {
			klog.V(4).Infof("pvc %s of cluster %s/%s skip reclaim, "+
				"cause pvc is still referenced by a pod", pvcName, namespace, ncName)
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("cluster %s/%s get pvc %s pod %s from cache failed: %v", namespace, ncName, pvcName, podName, err)
		}

		pvName := pvc.Spec.VolumeName
		if err := p.clientSet.PVC().DeletePVC(pvc.Namespace, pvcName); err != nil {
			return fmt.Errorf("cluster %s/%s delete pvc %s failed: %v", namespace, ncName, pvcName, err)
		}
		klog.Infof("cluster %s/%s reclaim pv %s success, pvc %s", namespace, ncName, pvName, pvcName)
	}
	return nil
}

func (p *pvcReclaimer) listPVCs(nc *v1alpha1.NebulaCluster) ([]corev1.PersistentVolumeClaim, error) {
	namespace := nc.GetNamespace()
	ncName := nc.GetName()

	selector, err := label.New().Cluster(nc.GetClusterName()).Selector()
	if err != nil {
		return nil, fmt.Errorf("get cluster %s/%s label selector failed: %v", namespace, ncName, err)
	}

	pvcs, err := p.clientSet.PVC().ListPVCs(namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("cluster %s/%s list pvc failed: %v", namespace, ncName, err)
	}
	return pvcs, nil
}

type FakePVCReclaimer struct {
	err error
}

func NewFakePVCReclaimer() *FakePVCReclaimer {
	return &FakePVCReclaimer{}
}

func (f *FakePVCReclaimer) SetReclaimError(err error) {
	f.err = err
}

func (f *FakePVCReclaimer) Reclaim(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
