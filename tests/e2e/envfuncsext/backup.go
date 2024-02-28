package envfuncsext

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	appspkg "github.com/vesoft-inc/nebula-operator/pkg/kube"
	e2econfig "github.com/vesoft-inc/nebula-operator/tests/e2e/config"
)

type (
	NebulaBackupInstallOptions struct {
		Name      string
		Namespace string
		Spec      appsv1alpha1.BackupSpec
	}

	nebulaBackupCtxKey struct {
		backupType string
	}

	NebulaBackupCtxValue struct {
		Name            string
		Namespace       string
		BackupFileName  string
		StorageType     string
		BucketName      string
		Region          string
		CleanBackupData bool
	}

	NebulaBackupOption  func(*NebulaBackupOptions)
	NebulaBackupOptions struct {
		WaitOptions []wait.Option
	}
)

func DeployNebulaBackup(incremental bool, nbCtx NebulaBackupInstallOptions) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		namespaceToUse := cfg.Namespace()
		if nbCtx.Namespace != "" {
			namespaceToUse = nbCtx.Namespace
		}

		nb := appsv1alpha1.NebulaBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nbCtx.Name,
				Namespace: namespaceToUse,
			},
			Spec: nbCtx.Spec,
		}

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		err = client.NebulaBackup().CreateNebulaBackup(&nb)
		if err != nil {
			return ctx, fmt.Errorf("error creating nebula backup [%v/%v]: %v", namespaceToUse, nbCtx.Name, err)
		}

		key := nebulaBackupCtxKey{backupType: "base"}
		if incremental {
			key = nebulaBackupCtxKey{backupType: "incr"}
		}

		var stoType, region, bucketName string
		if nb.Spec.Config.S3 != nil {
			stoType = "S3"
			region = nb.Spec.Config.S3.Region
			bucketName = nb.Spec.Config.S3.Bucket
		} else if nb.Spec.Config.GS != nil {
			stoType = "GS"
			region = nb.Spec.Config.GS.Location
			bucketName = nb.Spec.Config.GS.Bucket
		}

		return context.WithValue(ctx, key, &NebulaBackupCtxValue{
			Name:            nbCtx.Name,
			Namespace:       namespaceToUse,
			StorageType:     stoType,
			Region:          region,
			BucketName:      bucketName,
			CleanBackupData: *nb.Spec.CleanBackupData,
		}), nil
	}

}

func GetNebulaBackupCtxValue(incremental bool, ctx context.Context) *NebulaBackupCtxValue {
	key := nebulaBackupCtxKey{backupType: "base"}
	if incremental {
		key = nebulaBackupCtxKey{backupType: "incr"}
	}

	v := ctx.Value(key)
	data, _ := v.(*NebulaBackupCtxValue)
	return data
}

func WaitNebulaBackupFinished(incremental bool, opts ...NebulaBackupOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaBackupOptions{}).WithOptions(opts...)

		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		var nb *appsv1alpha1.NebulaBackup
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			nb, err = client.NebulaBackup().GetNebulaBackup(backupContextValue.Namespace, backupContextValue.Name)
			if err != nil {
				klog.ErrorS(err, "Get NebulaBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return false, err
			}
			klog.V(4).InfoS("Waiting for NebulaBackup to complete",
				"namespace", nb.Namespace, "name", nb.Name,
				"generation", nb.Generation,
			)

			if nb.Status.Phase == appsv1alpha1.BackupComplete {
				if backupContextValue.StorageType == "S3" {
					klog.V(4).Infof("Checking if backup file %v exists in S3 bucket %v", nb.Status.BackupName, nb.Spec.Config.S3.Bucket)

					exists, err := checkBackupExistsOnS3(ctx, nb.Spec.Config.S3.Region, nb.Spec.Config.S3.Bucket, nb.Status.BackupName)
					if err != nil {
						return true, fmt.Errorf("error checking if backup exists in S3 bucket: %v", err)
					}
					if !exists {
						return true, fmt.Errorf("backup has succeeded but a backup named %v was not found in S3 bucket %v", nb.Status.BackupName, nb.Spec.Config.S3.Bucket)
					}

					klog.V(4).Infof("Backup %v exists in S3 bucket %v", nb.Status.BackupName, nb.Spec.Config.S3.Bucket)
				} else if backupContextValue.StorageType == "GS" {
					klog.V(4).Infof("Checking if backup file %v exists in GS bucket %v", nb.Status.BackupName, nb.Spec.Config.GS.Bucket)

					exists, err := checkBackupExistsOnGS(ctx, nb.Spec.Config.GS.Location, nb.Spec.Config.GS.Bucket, nb.Status.BackupName)
					if err != nil {
						return true, fmt.Errorf("error checking if backup exists in GCP bucket: %v", err)
					}
					if !exists {
						return true, fmt.Errorf("backup has succeeded but a backup named %v was not found in GS bucket %v", nb.Status.BackupName, nb.Spec.Config.GS.Bucket)
					}

					klog.V(4).Infof("Backup %v exists in GS bucket %v", nb.Status.BackupName, nb.Spec.Config.GS.Bucket)
				}
				return true, nil
			}

			if nb.Status.Phase == appsv1alpha1.BackupFailed {
				return true, fmt.Errorf("nebula backup [%v/%v] has failed", nb.Namespace, nb.Name)
			}

			return false, nil
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaBackup to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
			return ctx, err
		}

		klog.InfoS("Waiting for NebulaCluster to be ready successfully", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "backup file name", nb.Status.BackupName)

		key := nebulaBackupCtxKey{backupType: "base"}
		if incremental {
			key = nebulaBackupCtxKey{backupType: "incr"}
		}

		return context.WithValue(ctx, key, &NebulaBackupCtxValue{
			Name:            backupContextValue.Name,
			Namespace:       backupContextValue.Namespace,
			BackupFileName:  nb.Status.BackupName,
			StorageType:     backupContextValue.StorageType,
			Region:          backupContextValue.Region,
			BucketName:      backupContextValue.BucketName,
			CleanBackupData: backupContextValue.CleanBackupData,
		}), nil
	}
}

func (o *NebulaBackupOptions) WithOptions(opts ...NebulaBackupOption) *NebulaBackupOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithNebulaBackupWaitOptions(opts ...wait.Option) NebulaBackupOption {
	return func(o *NebulaBackupOptions) {
		o.WaitOptions = append(o.WaitOptions, opts...)
	}
}

func DeleteNebulaBackup(incremental bool) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		err = client.NebulaBackup().DeleteNebulaBackup(backupContextValue.Namespace, backupContextValue.Name)
		if err != nil {
			return ctx, fmt.Errorf("error deleting nebula backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
		}

		return ctx, nil
	}
}

func WaitForCleanBackup(incremental bool, opts ...NebulaBackupOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaBackupOptions{}).WithOptions(opts...)

		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			nb, err := client.NebulaBackup().GetNebulaBackup(backupContextValue.Namespace, backupContextValue.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					if backupContextValue.CleanBackupData {
						if backupContextValue.StorageType == "S3" {
							ok, err := checkBackupExistsOnS3(ctx, backupContextValue.Region, backupContextValue.BucketName, backupContextValue.BackupFileName)
							if err != nil {
								return true, fmt.Errorf("error checking backup on S3: %v", err)
							}
							if ok {
								klog.V(4).Infof("backup %v is still in S3 bucket %v after 1st check. Will check again.", backupContextValue.BackupFileName, backupContextValue.BucketName)
								ok, err := checkBackupExistsOnS3(ctx, backupContextValue.Region, backupContextValue.BucketName, backupContextValue.BackupFileName)
								if err != nil {
									return true, fmt.Errorf("error checking backup on S3: %v", err)
								}
								if ok {
									return true, fmt.Errorf("backup %v is still in S3 bucket %v after 2nd check even though auto cleanup is enabled", backupContextValue.BackupFileName, backupContextValue.BucketName)
								}
							}
						} else if backupContextValue.StorageType == "GS" {
							ok, err := checkBackupExistsOnGS(ctx, backupContextValue.Region, backupContextValue.BucketName, backupContextValue.BackupFileName)
							if err != nil {
								return true, fmt.Errorf("error checking backup on GS: %v", err)
							}
							if ok {
								klog.V(4).Infof("backup %v is still in GS bucket %v after 1st check. Will check again.")
								ok, err := checkBackupExistsOnGS(ctx, backupContextValue.Region, backupContextValue.BucketName, backupContextValue.BackupFileName)
								if err != nil {
									return true, fmt.Errorf("error checking backup on GS: %v", err)
								}
								if ok {
									return true, fmt.Errorf("backup %v is still in GS bucket %v even though auto cleanup is enabled", backupContextValue.BackupFileName, backupContextValue.BucketName)
								}
							}
						}
					}
					return true, nil
				}
				return true, fmt.Errorf("error deleting nebula backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
			}

			if nb.Status.Phase == appsv1alpha1.BackupComplete || nb.Status.Phase == appsv1alpha1.BackupClean {
				klog.V(4).InfoS("Waiting for NebulaBackup cleanup to complete",
					"namespace", nb.Namespace, "name", nb.Name, "file name", backupContextValue.BackupFileName,
					"generation", nb.Generation,
				)
				return false, nil
			}

			return true, fmt.Errorf("nebula backup is in invalid phase %v", nb.Status.Phase)
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaBackup clean to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)
			return ctx, err
		}
		klog.InfoS("Waiting for NebulaCluster clean up to complete successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)

		return ctx, nil
	}
}

func checkBackupExistsOnS3(ctx context.Context, awsRegion, bucketName, backupName string) (bool, error) {
	creds := credentials.NewStaticCredentialsProvider(string(e2econfig.C.AWSAccessKey), string(e2econfig.C.AWSSecretKey), "")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(creds),
		config.WithRegion(awsRegion),
	)
	if err != nil {
		return false, fmt.Errorf("error loading AWS credentials: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(backupName),
	}

	result, err := client.ListObjectsV2(ctx, input)
	if err != nil {
		return false, fmt.Errorf("error listing objects: %v", err)
	}

	if len(result.Contents) == 0 {
		return false, nil
	}

	return true, nil
}

func checkBackupExistsOnGS(ctx context.Context, gsLocation, bucketName, backupName string) (bool, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(e2econfig.C.GSSecret))
	if err != nil {
		log.Fatalf("error creating GS client: %v", err)
	}
	defer client.Close()

	it := client.Bucket(bucketName).Objects(ctx, &storage.Query{
		Prefix: backupName + "/",
	})

	_, err = it.Next()
	if err == iterator.Done {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}
