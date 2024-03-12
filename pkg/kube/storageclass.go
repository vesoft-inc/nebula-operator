/*
Copyright 2024 Vesoft Inc.

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
	"errors"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrDefaultStorageClassNotFound = errors.New("default storageclass not found")

const IsDefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"

type StorageClass interface {
	GetStorageClass(scName string) (*storagev1.StorageClass, error)
	GetDefaultStorageClass() (*storagev1.StorageClass, error)
}

type scClient struct {
	client client.Client
}

func NewStorageClass(client client.Client) StorageClass {
	return &scClient{client: client}
}

func (c *scClient) GetStorageClass(scName string) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name: scName,
	}, sc)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get StorageClass", "name", scName)
		return nil, err
	}
	return sc, nil
}

func (c *scClient) GetDefaultStorageClass() (*storagev1.StorageClass, error) {
	scList := &storagev1.StorageClassList{}
	if err := c.client.List(context.TODO(), scList, &client.ListOptions{}); err != nil {
		return nil, err
	}
	for _, sc := range scList.Items {
		v, ok := sc.GetAnnotations()[IsDefaultStorageClassAnnotation]
		if ok && v == "true" {
			return &sc, nil
		}
	}
	return nil, ErrDefaultStorageClassNotFound
}
