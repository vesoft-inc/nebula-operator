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
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientSet interface {
	Node() Node
	Secret() Secret
	ConfigMap() ConfigMap
	PV() PersistentVolume
	PVC() PersistentVolumeClaim
	Pod() Pod
	Service() Service
	Ingress() Ingress
	Workload() Workload
	Deployment() Deployment
	NebulaCluster() NebulaCluster
	NebulaRestore() NebulaRestore
}

type clientSet struct {
	nodeClient     Node
	secretClient   Secret
	cmClient       ConfigMap
	pvClient       PersistentVolume
	pvcClient      PersistentVolumeClaim
	podClient      Pod
	svcClient      Service
	ingressClient  Ingress
	workloadClient Workload
	deployClient   Deployment
	nebulaClient   NebulaCluster
	restoreClient  NebulaRestore
}

func NewClientSet(config *rest.Config) (ClientSet, error) {
	cli, err := client.New(config, client.Options{})
	if err != nil {
		return nil, errors.Errorf("error building runtime client: %v", err)
	}
	return &clientSet{
		nodeClient:     NewNode(cli),
		secretClient:   NewSecret(cli),
		cmClient:       NewConfigMap(cli),
		pvClient:       NewPV(cli),
		pvcClient:      NewPVC(cli),
		podClient:      NewPod(cli),
		svcClient:      NewService(cli),
		ingressClient:  NewIngress(cli),
		workloadClient: NewWorkload(cli),
		deployClient:   NewDeployment(cli),
		nebulaClient:   NewNebulaCluster(cli),
		restoreClient:  NewNebulaRestore(cli),
	}, nil
}

func (c *clientSet) Node() Node {
	return c.nodeClient
}

func (c *clientSet) Secret() Secret {
	return c.secretClient
}

func (c *clientSet) ConfigMap() ConfigMap {
	return c.cmClient
}

func (c *clientSet) PV() PersistentVolume {
	return c.pvClient
}

func (c *clientSet) PVC() PersistentVolumeClaim {
	return c.pvcClient
}

func (c *clientSet) Pod() Pod {
	return c.podClient
}

func (c *clientSet) Service() Service {
	return c.svcClient
}

func (c *clientSet) Ingress() Ingress {
	return c.ingressClient
}

func (c *clientSet) Workload() Workload {
	return c.workloadClient
}

func (c *clientSet) Deployment() Deployment {
	return c.deployClient
}

func (c *clientSet) NebulaCluster() NebulaCluster {
	return c.nebulaClient
}

func (c *clientSet) NebulaRestore() NebulaRestore {
	return c.restoreClient
}
