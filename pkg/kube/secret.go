package kube

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Secret interface {
	GetSecret(namespace, secretName string) (*corev1.Secret, error)
}

type secretClient struct {
	kubecli client.Client
}

func NewSecret(kubecli client.Client) Secret {
	return &secretClient{kubecli: kubecli}
}

func (s *secretClient) GetSecret(namespace, secretName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := s.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}, secret)
	if err != nil {
		klog.Errorf("get secret [%s/%s] failed: %v", namespace, secretName, err)
		return nil, err
	}
	return secret, nil
}
