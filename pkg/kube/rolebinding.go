package kube

import (
	"context"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RoleBinding interface {
	CreateRoleBinding(roleBinding *rbacv1.RoleBinding) error
	GetRoleBinding(namespace, name string) (*rbacv1.RoleBinding, error)
}

type roleBindingClient struct {
	kubecli client.Client
}

func NewRoleBinding(kubecli client.Client) RoleBinding {
	return &roleBindingClient{kubecli: kubecli}
}

func (c *roleBindingClient) CreateRoleBinding(roleBinding *rbacv1.RoleBinding) error {
	return c.kubecli.Create(context.TODO(), roleBinding)
}

func (c *roleBindingClient) GetRoleBinding(namespace, name string) (*rbacv1.RoleBinding, error) {
	roleBinding := &rbacv1.RoleBinding{}
	err := c.kubecli.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, roleBinding)
	if err != nil {
		return nil, err
	}
	return roleBinding, nil
}
