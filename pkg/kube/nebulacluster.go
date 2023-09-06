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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type NebulaCluster interface {
	CreateNebulaCluster(nc *v1alpha1.NebulaCluster) error
	GetNebulaCluster(namespace, name string) (*v1alpha1.NebulaCluster, error)
	UpdateNebulaCluster(nc *v1alpha1.NebulaCluster) error
	UpdateNebulaClusterStatus(nc *v1alpha1.NebulaCluster) error
	DeleteNebulaCluster(namespace, name string) error
}

type nebulaClusterClient struct {
	client client.Client
}

func NewNebulaCluster(client client.Client) NebulaCluster {
	return &nebulaClusterClient{client: client}
}

func (c *nebulaClusterClient) CreateNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	if err := c.client.Create(context.TODO(), nc); err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("NebulaCluster %s/%s already exists", nc.Namespace, nc.Name)
			return nil
		}
		return err
	}
	return nil
}

func (c *nebulaClusterClient) GetNebulaCluster(namespace, name string) (*v1alpha1.NebulaCluster, error) {
	nebulaCluster := &v1alpha1.NebulaCluster{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, nebulaCluster)
	if err != nil {
		klog.Errorf("get nebulacluster [%s/%s] failed: %v", namespace, name, err)
		return nil, err
	}
	return nebulaCluster, nil
}

func (c *nebulaClusterClient) UpdateNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	ns := nc.Namespace
	ncName := nc.Name
	ncSpec := nc.Spec.DeepCopy()
	labels := nc.GetLabels()
	annotations := nc.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Update the set with the latest resource version for the next poll
		if updated, err := c.GetNebulaCluster(ns, ncName); err == nil {
			nc = updated.DeepCopy()
			nc.Spec = *ncSpec
			nc.SetLabels(labels)
			nc.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaCluster %s/%s failed: %v", ns, ncName, err))
			return err
		}

		updateErr := c.client.Update(context.TODO(), nc)
		if updateErr == nil {
			klog.Infof("NebulaCluster %s/%s updated successfully", ns, ncName)
			return nil
		}
		klog.Errorf("update NebulaCluster [%s/%s] failed: %v", ns, ncName, updateErr)
		return updateErr
	})
}

func (c *nebulaClusterClient) UpdateNebulaClusterStatus(nc *v1alpha1.NebulaCluster) error {
	ns := nc.Namespace
	ncName := nc.Name
	status := nc.Status.DeepCopy()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := c.GetNebulaCluster(ns, ncName); err == nil {
			// make a copy, so we don't mutate the shared cache
			nc = updated.DeepCopy()
			nc.Status = *status
		} else {
			utilruntime.HandleError(fmt.Errorf("get NebulaCluster [%s/%s] failed: %v", ns, ncName, err))
			return err
		}

		updateErr := c.client.Status().Update(context.TODO(), nc)
		if updateErr == nil {
			klog.Infof("NebulaCluster [%s/%s] updated successfully", ns, ncName)
			return nil
		}
		klog.Errorf("update NebulaCluster [%s/%s] status failed: %v", ns, ncName, updateErr)
		return updateErr
	})
}

func (c *nebulaClusterClient) DeleteNebulaCluster(namespace, name string) error {
	nc, err := c.GetNebulaCluster(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := c.client.Delete(context.TODO(), nc); err != nil {
		return err
	}
	klog.Infof("NebulaCluster [%s/%s] deleted successfully", namespace, name)
	return nil
}

type FakeNebulaCluster struct {
	client client.Client
}

func NewFakeNebulaCluster(client client.Client) NebulaCluster {
	return &FakeNebulaCluster{client: client}
}

func (f *FakeNebulaCluster) CreateNebulaCluster(_ *v1alpha1.NebulaCluster) error {
	return nil
}

func (f *FakeNebulaCluster) GetNebulaCluster(_, _ string) (*v1alpha1.NebulaCluster, error) {
	return nil, nil
}

func (f *FakeNebulaCluster) UpdateNebulaCluster(_ *v1alpha1.NebulaCluster) error {
	return nil
}

func (f *FakeNebulaCluster) UpdateNebulaClusterStatus(_ *v1alpha1.NebulaCluster) error {
	return nil
}

func (f *FakeNebulaCluster) DeleteNebulaCluster(_, _ string) error {
	return nil
}
