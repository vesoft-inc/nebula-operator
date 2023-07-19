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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	"github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

type ControlInterface interface {
	UpdateNebulaRestore(rt *v1alpha1.NebulaRestore) error
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

func (c *defaultRestoreControl) UpdateNebulaRestore(rt *v1alpha1.NebulaRestore) error {
	ns := rt.GetNamespace()
	name := rt.GetName()

	if condition.IsRestoreInvalid(rt) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is invalid.", ns, name)
		return nil
	}

	if condition.IsRestoreComplete(rt) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is complete.", ns, name)
		return nil
	}

	if condition.IsRestoreFailed(rt) {
		klog.Infof("Skipping because NebulaRestore [%s/%s] is failed.", ns, name)
		return nil
	}

	if rt.Status.ClusterName != "" {
		selector, err := label.New().Cluster(rt.Status.ClusterName).Selector()
		if err != nil {
			klog.Errorf("Fail to generate selector for NebulaCluster [%s/%s], %v", ns, rt.Status.ClusterName, err)
			return nil
		}
		pods, err := c.clientSet.Pod().ListPods(ns, selector)
		if err != nil {
			klog.Errorf("Fail to list pod for NebulaCluster [%s/%s] with selector %s, %v", ns, rt.Status.ClusterName, selector, err)
			return nil
		}
		for _, pod := range pods {
			if pod.Status.Phase == corev1.PodFailed {
				klog.Infof("NebulaCluster [%s/%s] has failed pod %s.", ns, name, pod.Name)
				if err := c.restoreManager.UpdateCondition(rt, &v1alpha1.RestoreCondition{
					Type:    v1alpha1.RestoreFailed,
					Status:  corev1.ConditionTrue,
					Reason:  "PodFailed",
					Message: fmt.Sprintf("Pod %s has failed", pod.Name),
				}); err != nil {
					klog.Errorf("Fail to update the condition of NebulaRestore [%s/%s], %v", ns, name, err)
				}
				if err := c.deleteRestoredCluster(ns, rt.Status.ClusterName); err != nil {
					klog.Errorf("Fail to delete NebulaCluster [%s/%s], %v", ns, rt.Status.ClusterName, err)
				}
				return nil
			}
		}
	}

	err := c.restoreManager.Sync(rt)
	if err != nil && !errors.IsReconcileError(err) {
		if err := c.restoreManager.UpdateCondition(rt, &v1alpha1.RestoreCondition{
			Type:    v1alpha1.RestoreFailed,
			Status:  corev1.ConditionTrue,
			Reason:  "ExecuteFailed",
			Message: err.Error(),
		}); err != nil {
			klog.Errorf("Fail to update the condition of NebulaRestore [%s/%s], %v", ns, name, err)
		}
		updated, err := c.clientSet.NebulaRestore().GetNebulaRestore(ns, rt.Name)
		if err != nil {
			klog.Errorf("Fail to get NebulaRestore [%s/%s], %v", ns, name, err)
		}
		if err := c.deleteRestoredCluster(ns, updated.Status.ClusterName); err != nil {
			klog.Errorf("Fail to delete NebulaCluster %v", err)
		}
		return nil
	}

	return err
}

func (c *defaultRestoreControl) deleteRestoredCluster(namespace, ncName string) error {
	return c.clientSet.NebulaCluster().DeleteNebulaCluster(namespace, ncName)
}
