package kube

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccount interface {
	CreateServiceAccount(serviceAccount *v1.ServiceAccount) error
	GetServiceAccount(namespace, name string) (*v1.ServiceAccount, error)
}

type serviceAccountClient struct {
	kubecli client.Client
}

func NewServiceAccount(kubecli client.Client) ServiceAccount {
	return &serviceAccountClient{kubecli: kubecli}
}

func (c *serviceAccountClient) CreateServiceAccount(serviceAccount *v1.ServiceAccount) error {
	return c.kubecli.Create(context.TODO(), serviceAccount)
}

func (c *serviceAccountClient) GetServiceAccount(namespace, name string) (*v1.ServiceAccount, error) {
	serviceAccount := &v1.ServiceAccount{}
	err := c.kubecli.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, serviceAccount)
	if err != nil {
		return nil, err
	}
	return serviceAccount, nil
}
