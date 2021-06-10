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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type NebulaCluster interface {
	GetNebulaCluster(namespace, name string) (*v1alpha1.NebulaCluster, error)
	UpdateNebulaClusterStatus(nc *v1alpha1.NebulaCluster) error
}

type nebulaClusterClient struct {
	cli client.Client
}

func NewNebulaCluster(cli client.Client) NebulaCluster {
	return &nebulaClusterClient{cli: cli}
}

func (c *nebulaClusterClient) GetNebulaCluster(namespace, name string) (*v1alpha1.NebulaCluster, error) {
	nebulaCluster := &v1alpha1.NebulaCluster{}
	err := c.cli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, nebulaCluster)
	if err != nil {
		return nil, err
	}
	return nebulaCluster, nil
}

func (c *nebulaClusterClient) UpdateNebulaClusterStatus(nc *v1alpha1.NebulaCluster) error {
	log := getLog().WithValues("namespace", nc.Namespace, "name", nc.Name)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return c.cli.Status().Update(context.TODO(), nc)
	})
	if err != nil {
		return err
	}
	log.Info("nebulaCluster updated")
	return nil
}

type FakeNebulaCluster struct {
	cli client.Client
}

func NewFakeNebulaCluster(cli client.Client) NebulaCluster {
	return &FakeNebulaCluster{cli: cli}
}

func (f *FakeNebulaCluster) GetNebulaCluster(namespace, name string) (*v1alpha1.NebulaCluster, error) {
	nebulaCluster := &v1alpha1.NebulaCluster{}
	err := f.cli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, nebulaCluster)
	if err != nil {
		return nil, err
	}
	return nebulaCluster, nil
}

func (f *FakeNebulaCluster) UpdateNebulaClusterStatus(nc *v1alpha1.NebulaCluster) error {
	return f.cli.Status().Update(context.TODO(), nc)
}
