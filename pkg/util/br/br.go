package br

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

const (
	S3AccessKey = "access-key"
	S3SecretKey = "secret-key"
)

func GetS3Key(clientSet kube.ClientSet, namespace, secretName string) (accessKey string, secretKey string, err error) {
	var secret *corev1.Secret
	secret, err = clientSet.Secret().GetSecret(namespace, secretName)
	if err != nil {
		return
	}
	accessKey = string(secret.Data[S3AccessKey])
	secretKey = string(secret.Data[S3SecretKey])

	return
}
