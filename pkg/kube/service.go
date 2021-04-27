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
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service interface {
	CreateService(service *corev1.Service) error
	GetService(namespace string, name string) (*corev1.Service, error)
	UpdateService(service *corev1.Service) error
	DeleteService(namespace string, name string) error
}

type serviceClient struct {
	kubecli client.Client
}

func NewService(kubecli client.Client) Service {
	return &serviceClient{kubecli: kubecli}
}

func (s *serviceClient) CreateService(service *corev1.Service) error {
	if err := s.kubecli.Create(context.TODO(), service); err != nil {
		klog.Errorf(" %s name %s already exists", service.Namespace, service.Name)
		return nil
	}
	klog.Infof("namespace %s service %s created", service.Namespace, service.Name)
	return nil
}

func (s *serviceClient) GetService(namespace, name string) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := s.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, service)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (s *serviceClient) UpdateService(service *corev1.Service) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := s.kubecli.Update(context.TODO(), service); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	klog.Infof("namespace %s service %s updated", service.Namespace, service.Name)
	return nil
}

func (s *serviceClient) DeleteService(namespace, name string) error {
	service := &corev1.Service{}
	err := s.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, service)
	if err != nil {
		return err
	}
	klog.Infof("namespace %s service %s deleted", namespace, name)
	return s.kubecli.Delete(context.TODO(), service)
}
