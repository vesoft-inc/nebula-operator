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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/envfuncsext"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

const (
	LabelCategoryBackup = "backup"
	LabelGroupBasic     = "basic"
)

var testCasesBackup []ncTestCase

func init() {
	testCasesBackup = append(testCasesBackup, testCasesBasicBackup...)
}

// helper variables
var enable, disable bool = true, false

var storageS3 = appsv1alpha1.StorageProvider{
	S3: &appsv1alpha1.S3StorageProvider{
		Region:     AWSRegion,
		Bucket:     AWSBucketName,
		Endpoint:   AWSBucketEndpoint,
		SecretName: AWSSecretName,
	},
}

var storageGS = appsv1alpha1.StorageProvider{
	GS: &appsv1alpha1.GsStorageProvider{
		Location:   GSLocation,
		Bucket:     GSBucketName,
		SecretName: GSSecretName,
	},
}

var basicBackupSpec = appsv1alpha1.BackupSpec{
	Image:   config.C.NebulaGraph.BrImage,
	Version: config.C.NebulaGraph.BrVersion,
	Resources: corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("300Mi"),
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("200Mi"),
		},
	},
	ImagePullSecrets: []corev1.LocalObjectReference{
		{
			Name: ImagePullSecretName,
		},
	},
	AutoRemoveFinished: &disable,
	CleanBackupData:    &disable,
	Config: &appsv1alpha1.BackupConfig{
		StorageProvider: storageS3,
	},
}

var cronBackupOps = envfuncsext.NebulaCronBackupOptions{
	Schedule:  "* * * * *",
	TestPause: true,
}

var basicRestoreSpec = appsv1alpha1.RestoreSpec{
	AutoRemoveFailed: disable,
	Config: &appsv1alpha1.RestoreConfig{
		Concurrency:     3,
		StorageProvider: storageS3,
	},
}

// test cases about backup
var testCasesBasicBackup = []ncTestCase{
	{
		Name: "NebulaGraph Backup Basic Tests",
		Labels: map[string]string{
			LabelKeyCategory: LabelCategoryBackup,
			LabelKeyGroup:    LabelGroupBasic,
		},
		InstallNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterHelmRawOptions(
				helm.WithArgs(
					"--set", "nebula.enableBR=true",
				),
			),
		},
		InstallWaitNCOptions: []envfuncsext.NebulaClusterOption{
			envfuncsext.WithNebulaClusterReadyFuncs(
				envfuncsext.NebulaClusterReadyFuncForFields(false, map[string]any{
					"Spec": map[string]any{
						"Graphd": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(2),
						},
						"Metad": map[string]any{
							"Replicas": e2ematcher.ValidatorEq(3),
						},
						"Storaged": map[string]any{
							"Replicas":          e2ematcher.ValidatorEq(3),
							"EnableAutoBalance": e2ematcher.ValidatorEq(false),
						},
					},
				}),
				envfuncsext.DefaultNebulaClusterReadyFunc,
			),
		},
		LoadLDBC:     true,
		UpgradeCases: nil,
		BackupCases: []ncBackupCase{
			{
				Name: "basic backup with S3",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-basic-s3",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-basic-s3",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "basic backup with S3 and auto delete",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-basic-s3-auto",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"autoRemoveFinished": &enable,
					"cleanBackupData":    &enable,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-basic-s3-auto",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "incremental backup with S3",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-incr-s3",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				Incremental: true,
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-incr-s3",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "basic backup with S3 across namespaces",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:      "nb-basic-ns-s3",
					Namespace: "default",
					Spec:      *basicBackupSpec.DeepCopy(),
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name:      "nb-basic-ns-s3",
					Namespace: "default",
					Spec:      *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "incremental backup with S3 across namespaces",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:      "nb-incr-ns-s3",
					Namespace: "default",
					Spec:      *basicBackupSpec.DeepCopy(),
				},
				Incremental: true,
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name:      "nb-incr-ns-s3",
					Namespace: "default",
					Spec:      *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "cron backup with S3",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:          "nb-cron-s3",
					Spec:          *basicBackupSpec.DeepCopy(),
					CronBackupOps: &cronBackupOps,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-cron-s3",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "basic backup with gs",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-basic-gs",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"storageProvider": storageGS,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-basic-gs",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "basic backup with gs and auto delete",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-basic-gs-auto",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"storageProvider":    storageGS,
					"autoRemoveFinished": &enable,
					"cleanBackupData":    &enable,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-basic-gs-auto",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "incremental backup with gs",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name: "nb-incr-gs-auto",
					Spec: *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"autoRemoveFinished": &enable,
					"cleanBackupData":    &enable,
					"storageProvider":    storageGS,
				},
				Incremental: true,
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-incr-gs-auto",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "basic backup with gs across namespaces",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:      "nb-basic-ns-gs",
					Namespace: "default",
					Spec:      *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"storageProvider": storageGS,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-basic-ns-gs",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "incr backup with gs across namespaces",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:      "nb-incr-ns-gs",
					Namespace: "default",
					Spec:      *basicBackupSpec.DeepCopy(),
				},
				BackupUpdateOptions: map[string]any{
					"storageProvider": storageGS,
				},
				Incremental: true,
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-incr-ns-gs",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
			{
				Name: "cron backup with GCP",
				BackupInstallOptions: envfuncsext.NebulaBackupInstallOptions{
					Name:          "nb-cron-gcp",
					Spec:          *basicBackupSpec.DeepCopy(),
					CronBackupOps: &cronBackupOps,
				},
				BackupUpdateOptions: map[string]any{
					"storageProvider": storageGS,
				},
				RestoreInstallOptions: &envfuncsext.NebulaRestoreInstallOptions{
					Name: "nb-cron-gcp",
					Spec: *basicRestoreSpec.DeepCopy(),
				},
			},
		},
	},
}
