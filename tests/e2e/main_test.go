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

package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"
)

const (
	ImagePullSecretName = "image-pull-secret.e2e"
	AWSSecretName       = "aws-secret.e2e"
	AWSRegion           = "us-east-2"
	AWSBucketName       = "nebula-e2e"
	AWSBucketEndpoint   = "https://s3.us-east-2.amazonaws.com"

	GSLocation   = "us-east-5"
	GSSecretName = "gs-secret"
	GSBucketName = "nebula-cloud-e2e-test-bucket"

	LabelKeyCategory = "category"
	LabelKeyGroup    = "group"
)

var (
	testEnv env.Environment
)

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to build envconf from flags: %s", err)
	}

	testEnv = env.NewWithConfig(cfg)

	// ############################## Setup START ##############################
	// Setup Kubernetes Cluster
	klog.V(4).InfoS("Setup Kubernetes Cluster")

	testEnv.Setup(
		envfuncsext.InstallCluster(
			envfuncsext.WithClusterNamePrefix("e2e"),
			envfuncsext.WithClusterKindConfigPath(config.C.Cluster.KindConfigPath),
		),
	)

	// Setup Kubernetes scheme
	testEnv.Setup(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if err = appsv1alpha1.AddToScheme(cfg.Client().Resources().GetScheme()); err != nil {
			klog.ErrorS(err, "Setup kubernetes scheme failed")
			return ctx, err
		}
		return ctx, nil
	})

	// Setup Nebula Operator
	if config.C.Operator.Install {
		testEnv.Setup(
			envfuncsext.CreateObject(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.C.Operator.Namespace}}),
		)

		operatorOptions := []envfuncsext.OperatorOption{
			envfuncsext.WithOperatorHelmRawOptions(
				helm.WithChart(config.C.Operator.ChartPath),
				helm.WithNamespace(config.C.Operator.Namespace),
				helm.WithName(config.C.Operator.Name),
				helm.WithArgs("--set", fmt.Sprintf("image.nebulaOperator.image=%s", config.C.Operator.Image)),
			),
		}

		if config.C.Operator.Image != "" {
			klog.V(3).InfoS("Setup Nebula Operator image", "image", config.C.Operator.Image, "namespace", config.C.Operator.Namespace, "name", config.C.Operator.Name)
			operatorOptions = append(operatorOptions,
				envfuncsext.WithOperatorHelmRawOptions(
					helm.WithArgs("--set", fmt.Sprintf("image.nebulaOperator.image=%s", config.C.Operator.Image)),
				),
			)
		}

		if len(config.C.DockerConfigJsonSecret) > 0 {
			klog.V(3).InfoS("Setup Nebula Operator docker config json", "namespace", config.C.Operator.Namespace, "name", config.C.Operator.Name)
			operatorOptions = append(operatorOptions,
				envfuncsext.WithOperatorHelmRawOptions(
					helm.WithArgs("--set", fmt.Sprintf("imagePullSecrets[0].name=%s", ImagePullSecretName)),
				),
			)

			testEnv.Setup(envfuncsext.CreateObject(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ImagePullSecretName,
						Namespace: config.C.Operator.Namespace,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: config.C.DockerConfigJsonSecret,
					},
				},
			))
		}

		klog.V(4).InfoS("Setup Nebula Operator", "namespace", config.C.Operator.Namespace, "name", config.C.Operator.Name)
		testEnv.Setup(
			envfuncsext.InstallOperator(operatorOptions...),
		)
	}
	// ############################## Setup END    ##############################

	// ############################## Finish START ##############################
	// Finish Nebula Operator
	if config.C.Operator.Install {
		testEnv.Finish(
			envfuncsext.UninstallOperator(
				envfuncsext.WithOperatorHelmRawOptions(
					helm.WithNamespace(config.C.Operator.Namespace),
					helm.WithName(config.C.Operator.Name),
				),
			),
			envfuncsext.DeleteObject(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.C.Operator.Namespace}}),
		)
	}

	// Finish Kubernetes Cluster
	testEnv.Finish(
		envfuncsext.UninstallCluster(),
	)

	// ############################## Finish END    ##############################

	os.Exit(testEnv.Run(m))
}
