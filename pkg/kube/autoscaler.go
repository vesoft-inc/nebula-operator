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

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/v1alpha1"
)

type NebulaAutoscaler interface {
	GetNebulaAutoscaler(namespace, name string) (*v1alpha1.NebulaAutoscaler, error)
	UpdateNebulaAutoscalerStatus(na *v1alpha1.NebulaAutoscaler) error
}

type nebulaAutoscalerClient struct {
	client client.Client
}

func NewNebulaAutoscaler(client client.Client) NebulaAutoscaler {
	return &nebulaAutoscalerClient{client: client}
}

func (c *nebulaAutoscalerClient) GetNebulaAutoscaler(namespace, name string) (*v1alpha1.NebulaAutoscaler, error) {
	nebulaCluster := &v1alpha1.NebulaAutoscaler{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, nebulaCluster)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaAutoscaler", "namespace", namespace, "name", name)
		return nil, err
	}
	return nebulaCluster, nil
}

func (c *nebulaAutoscalerClient) UpdateNebulaAutoscalerStatus(na *v1alpha1.NebulaAutoscaler) error {
	ns := na.Namespace
	ncName := na.Name
	status := na.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := c.GetNebulaAutoscaler(ns, ncName); err == nil {
			// make a copy, so we don't mutate the shared cache
			na = updated.DeepCopy()
			na.Status = *status
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaAutoscaler [%s/%s] failed: %v", ns, ncName, err))
			return err
		}

		updateErr := c.client.Status().Update(context.TODO(), na)
		if updateErr == nil {
			klog.Infof("NebulaAutoscaler [%s/%s] updated successfully", ns, ncName)
			return nil
		}
		klog.Errorf("update NebulaAutoscaler [%s/%s] status failed: %v", ns, ncName, updateErr)
		return updateErr
	})
}
