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

package credentials

import (
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

const (
	AWSAccessKey    = "access_key"
	AWSSecretKey    = "secret_key"
	AWSAccessKeyEnv = "AWS_ACCESS_KEY_ID"
	AWSSecretKeyEnv = "AWS_SECRET_ACCESS_KEY"

	GSCredentialsKey = "credentials"
	GSCredentialsEnv = "GOOGLE_APPLICATION_CREDENTIALS_JSON"
)

func GetStorageType(provider v1alpha1.StorageProvider) v1alpha1.ObjectStorageType {
	if provider.S3 != nil {
		return v1alpha1.ObjectStorageS3
	}
	if provider.GS != nil {
		return v1alpha1.ObjectStorageGS
	}
	return v1alpha1.ObjectStorageUnknown
}

func GetGsCredentials(clientSet kube.ClientSet, namespace, secretName string) (credentials string, err error) {
	if os.Getenv(GSCredentialsEnv) != "" {
		credentials = os.Getenv(GSCredentialsEnv)
		return
	}

	var secret *corev1.Secret
	secret, err = clientSet.Secret().GetSecret(namespace, secretName)
	if err != nil {
		return
	}
	credentials = string(secret.Data[GSCredentialsKey])
	return
}

func GetS3Key(clientSet kube.ClientSet, namespace, secretName string) (accessKey string, secretKey string, err error) {
	if os.Getenv(AWSAccessKeyEnv) != "" && os.Getenv(AWSSecretKeyEnv) != "" {
		accessKey = os.Getenv(AWSAccessKeyEnv)
		secretKey = os.Getenv(AWSSecretKeyEnv)
		return
	}

	var secret *corev1.Secret
	secret, err = clientSet.Secret().GetSecret(namespace, secretName)
	if err != nil {
		return
	}
	accessKey = string(secret.Data[AWSAccessKey])
	secretKey = string(secret.Data[AWSSecretKey])

	return
}
