package envfuncsext

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type (
	NebulaBackupInstallOptions struct {
		Name          string
		Namespace     string
		Spec          appsv1alpha1.BackupSpec
		CronBackupOps *NebulaCronBackupOptions
	}

	NebulaCronBackupOptions struct {
		Schedule  string
		TestPause bool
	}

	nebulaBackupCtxKey struct {
		backupType string
	}

	NebulaBackupCtxValue struct {
		// general fields
		Name            string
		Namespace       string
		BackupFileName  string
		StorageType     string
		BucketName      string
		Region          string
		CleanBackupData bool

		// for cron backup only
		Schedule            string
		TestPause           bool
		TriggeredBackupName string
		BackupSpec          appsv1alpha1.BackupSpec
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

		nb := &appsv1alpha1.NebulaBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nbCtx.Name,
				Namespace: namespaceToUse,
			},
			Spec: nbCtx.Spec,
		}

		ctx, err := CreateObject(nb)(ctx, cfg)
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

		nb := &appsv1alpha1.NebulaBackup{}
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			err = cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, nb)
			if err != nil {
				klog.ErrorS(err, "Get NebulaBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return true, err
			}

			klog.V(4).InfoS("Waiting for NebulaBackup to complete",
				"namespace", nb.Namespace, "name", nb.Name,
				"generation", nb.Generation,
			)

			if nb.Status.Phase == appsv1alpha1.BackupComplete {
				return true, nil
			}

			if nb.Status.Phase == appsv1alpha1.BackupFailed || nb.Status.Phase == appsv1alpha1.BackupInvalid {
				return true, fmt.Errorf("nebula backup [%v/%v] has failed", nb.Namespace, nb.Name)
			}

			return false, nil
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaBackup to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
			return ctx, err
		}

		klog.InfoS("Waiting for NebulaBackup to complete successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "backup file name", nb.Status.BackupName)

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

		nb := &appsv1alpha1.NebulaBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupContextValue.Name,
				Namespace: backupContextValue.Namespace,
			},
		}

		ctx, err := DeleteObject(nb)(ctx, cfg)
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

		nb := &appsv1alpha1.NebulaBackup{}
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			err = cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, nb)
			if err != nil {
				if apierrors.IsNotFound(err) {
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

			return true, fmt.Errorf("nebula backup clean for [%v/%v] has failed", backupContextValue.Namespace, backupContextValue.Name)
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaBackup clean to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)
			return ctx, err
		}
		klog.InfoS("Waiting for NebulaBackup clean to complete successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)

		return ctx, nil
	}
}

func CreateServiceAccount(namespace, name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		if ctx, err := CreateObject(sa)(ctx, cfg); err != nil {
			if apierrors.IsAlreadyExists(err) {
				klog.Infof("service account [%s/%s] already exists", sa.Namespace, sa.Name)
				return ctx, nil
			}
			return ctx, err
		}

		klog.Infof("Service account [%s/%s] created successfully", namespace, name)
		return ctx, nil
	}
}

func DeleteServiceAccount(namespace, name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		if err := cfg.Client().Resources().Delete(ctx, sa); err != nil {
			return ctx, err
		}

		klog.Infof("Service account [%s/%s] deleted successfully", namespace, name)
		return ctx, nil
	}
}
