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
	"fmt"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

func PvcGc(cli client.Client, namespace, clusterName string) error {
	selector, err := label.New().Cluster(clusterName).Selector()
	if err != nil {
		return fmt.Errorf("get cluster %s/%s label selector failed: %v", namespace, clusterName, err)
	}

	pvcClient := kube.NewPVC(cli)
	pvcs, err := pvcClient.ListPVCs(namespace, selector)
	if err != nil {
		return fmt.Errorf("cluster %s/%s list pvc failed: %v", namespace, clusterName, err)
	}

	for i := range pvcs {
		pvc := pvcs[i]
		if err := pvcClient.DeletePVC(pvc.Namespace, pvc.Name); err != nil {
			return err
		}
	}

	return nil
}

func PvcMark(pvcClient kube.PersistentVolumeClaim, component v1alpha1.NebulaClusterComponentter, oldReplicas, newReplicas int32) error {
	componentName := component.GetName()
	for i := oldReplicas - 1; i >= newReplicas; i-- {
		pvcName := ordinalPVCName(component.Type(), componentName, i)
		pvc, err := pvcClient.GetPVC(component.GetNamespace(), pvcName)
		if err != nil {
			return fmt.Errorf("get pvc %s for cluster %s/%s failed: %s", pvcName, component.GetNamespace(), component.GetClusterName(), err)
		}

		if pvc.Annotations == nil {
			pvc.Annotations = map[string]string{}
		}
		pvc.Annotations[annotation.AnnPVCDeferDeletingKey] = time.Now().Format(time.RFC3339)

		if err := pvcClient.UpdatePVC(pvc); err != nil {
			klog.Errorf("failed to set pvc %s/%s annotation: %s", component.GetNamespace(), pvcName, annotation.AnnPVCDeferDeletingKey)
			return err
		}
		klog.Infof("set pvc %s/%s annotation: %s succeed", component.GetNamespace(), pvcName, annotation.AnnPVCDeferDeletingKey)
	}
	return nil
}

func ordinalPVCName(componentType v1alpha1.ComponentType, setName string, ordinal int32) string {
	return fmt.Sprintf("%s-%s-%d", componentType, setName, ordinal)
}
