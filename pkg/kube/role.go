package kube

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Role interface {
	CreateRole(role *rbacv1.Role) error
	GetRole(namespace, name string) (*rbacv1.Role, error)
}

type roleClient struct {
	kubecli client.Client
}

func NewRole(kubecli client.Client) Role {
	return &roleClient{kubecli: kubecli}
}

func (c *roleClient) CreateRole(role *rbacv1.Role) error {
	return c.kubecli.Create(context.TODO(), role)
}

func (c *roleClient) GetRole(namespace, name string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{}
	err := c.kubecli.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, role)
	if err != nil {
		return nil, err
	}
	return role, nil
}
