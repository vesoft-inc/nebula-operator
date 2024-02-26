/*
Copyright 2023 Vesoft Inc.

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

package nebularestore

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

type ControlInterface interface {
	UpdateNebulaRestore(nr *v1alpha1.NebulaRestore) error
}

var _ ControlInterface = (*defaultRestoreControl)(nil)

type defaultRestoreControl struct {
	clientSet      kube.ClientSet
	restoreManager Manager
}

func NewRestoreControl(clientSet kube.ClientSet, restoreManager Manager) ControlInterface {
	return &defaultRestoreControl{
		clientSet:      clientSet,
		restoreManager: restoreManager,
	}
}

func (c *defaultRestoreControl) UpdateNebulaRestore(nr *v1alpha1.NebulaRestore) error {
	ns := nr.GetNamespace()
	name := nr.GetName()

	if condition.IsRestoreInvalid(nr) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is invalid.", ns, name)
		return nil
	}

	if condition.IsRestoreComplete(nr) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is complete.", ns, name)
		return nil
	}

	if condition.IsRestoreFailed(nr) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is failed.", ns, name)
		return nil
	}

	if nr.Status.ClusterName != "" {
		selector, err := label.New().Cluster(nr.Status.ClusterName).Selector()
		if err != nil {
			klog.Errorf("Fail to generate selector for NebulaCluster [%s/%s], %v", ns, nr.Status.ClusterName, err)
			return nil
		}
		pods, err := c.clientSet.Pod().ListPods(ns, selector)
		if err != nil {
			klog.Errorf("Fail to list pod for NebulaCluster [%s/%s] with selector %s, %v", ns, nr.Status.ClusterName, selector, err)
			return nil
		}
		for _, pod := range pods {
			if pod.Status.Phase == corev1.PodFailed {
				klog.Infof("NebulaCluster [%s/%s] has failed pod %s.", ns, name, pod.Name)
				if err := c.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr, &v1alpha1.RestoreCondition{
					Type:    v1alpha1.RestoreFailed,
					Status:  corev1.ConditionTrue,
					Reason:  "PodFailed",
					Message: getPodTerminateReason(pod),
				}, &kube.RestoreUpdateStatus{
					ConditionType: v1alpha1.RestoreFailed,
				}); err != nil {
					klog.Errorf("Fail to update the condition of NebulaRestore [%s/%s], %v", ns, name, err)
				}
				if nr.Spec.AutoRemoveFailed {
					if err := c.deleteRestoredCluster(ns, nr.Status.ClusterName); err != nil {
						klog.Errorf("Fail to delete NebulaCluster [%s/%s], %v", ns, nr.Status.ClusterName, err)
					}
				}
				return nil
			}
		}
	}

	err := c.restoreManager.Sync(nr)
	if err != nil && !utilerrors.IsReconcileError(err) {
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err := c.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr, &v1alpha1.RestoreCondition{
			Type:    v1alpha1.RestoreFailed,
			Status:  corev1.ConditionTrue,
			Reason:  "ExecuteFailed",
			Message: err.Error(),
		}, &kube.RestoreUpdateStatus{
			ConditionType: v1alpha1.RestoreFailed,
		}); err != nil {
			klog.Errorf("Fail to update the condition of NebulaRestore [%s/%s], %v", ns, name, err)
		}
		updated, err := c.clientSet.NebulaRestore().GetNebulaRestore(ns, nr.Name)
		if err != nil {
			klog.Errorf("Fail to get NebulaRestore [%s/%s], %v", ns, name, err)
		}
		if nr.Spec.AutoRemoveFailed {
			if err := c.deleteRestoredCluster(ns, updated.Status.ClusterName); err != nil {
				klog.Errorf("Fail to delete NebulaCluster %v", err)
			}
		}
		return nil
	}

	return err
}

func (c *defaultRestoreControl) deleteRestoredCluster(namespace, ncName string) error {
	return c.clientSet.NebulaCluster().DeleteNebulaCluster(namespace, ncName)
}
