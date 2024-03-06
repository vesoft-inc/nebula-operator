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

package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Node interface {
	GetNode(nodeName string) (*corev1.Node, error)
	ListAllNodes() ([]corev1.Node, error)
}

type nodeClient struct {
	kubecli client.Client
}

func NewNode(kubecli client.Client) Node {
	return &nodeClient{kubecli: kubecli}
}

func (pd *nodeClient) GetNode(nodeName string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := pd.kubecli.Get(context.TODO(), types.NamespacedName{Name: nodeName}, node)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get node", "node", nodeName)
		return nil, err
	}
	return node, nil
}

func (pd *nodeClient) ListAllNodes() ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := pd.kubecli.List(context.TODO(), nodeList, &client.ListOptions{}); err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}
