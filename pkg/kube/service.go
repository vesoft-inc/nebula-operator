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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
	log := getLog().WithValues("namespace", service.Namespace, "name", service.Name)
	if err := s.kubecli.Create(context.TODO(), service); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("service already exists")
			return nil
		}
		log.Error(err, "service created failed")
		return err
	}
	log.Info("service created")
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
	log := getLog().WithValues("namespace", service.Namespace, "name", service.Name)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return s.kubecli.Update(context.TODO(), service)
	})
	if err != nil {
		return err
	}
	log.Info("service updated")
	return nil
}

func (s *serviceClient) DeleteService(namespace, name string) error {
	log := getLog().WithValues("namespace", namespace, "name", name)
	service := &corev1.Service{}
	err := s.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, service)
	if err != nil {
		return err
	}

	log.Info("service deleted")
	return s.kubecli.Delete(context.TODO(), service)
}
