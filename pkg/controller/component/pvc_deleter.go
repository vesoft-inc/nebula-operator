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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

func PVCDeleter(cli client.Client, namespace, clusterName string) error {
	selector, err := label.New().Cluster(clusterName).Selector()
	if err != nil {
		return fmt.Errorf("get cluster [%s/%s] label selector failed: %v", namespace, clusterName, err)
	}

	pvcClient := kube.NewPVC(cli)
	pvcs, err := pvcClient.ListPVCs(namespace, selector)
	if err != nil {
		return fmt.Errorf("cluster [%s/%s] list PVC failed: %v", namespace, clusterName, err)
	}

	for i := range pvcs {
		pvc := pvcs[i]
		if pvc.Annotations[annotation.AnnPvReclaimKey] == "false" {
			continue
		}
		if err := pvcClient.DeletePVC(pvc.Namespace, pvc.Name); err != nil {
			return err
		}
	}

	return nil
}

func PVCMark(pvcClient kube.PersistentVolumeClaim, component v1alpha1.NebulaClusterComponent, oldReplicas, newReplicas int32) error {
	ns := component.GetNamespace()
	componentName := component.GetName()
	for i := oldReplicas - 1; i >= newReplicas; i-- {
		pvcNames := ordinalPVCNames(component.ComponentType(), componentName, i)
		for _, pvcName := range pvcNames {
			pvc, err := pvcClient.GetPVC(component.GetNamespace(), pvcName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("get PVC %s for cluster [%s/%s] failed: %s",
					pvcName, component.GetNamespace(), component.GetClusterName(), err)
			}

			if pvc.Annotations == nil {
				pvc.Annotations = map[string]string{}
			}
			now := time.Now().Format(time.RFC3339)
			pvc.Annotations[annotation.AnnPVCDeferDeletingKey] = now
			if err := pvcClient.UpdatePVC(pvc); err != nil {
				klog.Errorf("component [%s/%s] set PVC %s annotations failed: %v", ns, componentName, pvcName, err)
				return err
			}
			klog.Infof("component [%s/%s] set PVC %s annotations succeed", ns, componentName, pvcName)
		}
	}
	return nil
}

func ordinalPVCNames(componentType v1alpha1.ComponentType, setName string, ordinal int32) []string {
	// Todo: here need a unified function to get logPVC and dataPVC name
	logPVC := fmt.Sprintf("%s-log-%s-%d", componentType, setName, ordinal)
	dataPVC := fmt.Sprintf("%s-data-%s-%d", componentType, setName, ordinal)
	if componentType == v1alpha1.GraphdComponentType {
		return []string{logPVC}
	}
	return []string{logPVC, dataPVC}
}
