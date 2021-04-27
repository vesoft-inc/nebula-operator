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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Endpoint interface {
	GetEndpoints(namespace string, name string) (*corev1.Endpoints, error)
}

type endpointClient struct {
	kubecli client.Client
}

func NewEndpointClient(cli client.Client) Endpoint {
	return &endpointClient{kubecli: cli}
}

func (e *endpointClient) GetEndpoints(namespace, name string) (*corev1.Endpoints, error) {
	endpoints := &corev1.Endpoints{}
	if err := e.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}
