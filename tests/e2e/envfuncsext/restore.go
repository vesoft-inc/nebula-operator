package envfuncsext

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	appspkg "github.com/vesoft-inc/nebula-operator/pkg/kube"
)

type (
	NebulaRestoreInstallOptions struct {
		Name      string
		Namespace string
		Spec      appsv1alpha1.RestoreSpec
	}

	nebulaRestoreCtxKey struct{}

	NebulaRestoreCtxValue struct {
		Name                    string
		Namespace               string
		BackupFileName          string
		StorageType             string
		BucketName              string
		RestoreClusterNamespace string
		RestoreClusterName      string
	}

	NebulaRestoreOption  func(*NebulaRestoreOptions)
	NebulaRestoreOptions struct {
		WaitOptions []wait.Option
	}
)

func DeployNebulaRestore(nbCtx NebulaRestoreInstallOptions) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		namespaceToUse := cfg.Namespace()
		if nbCtx.Namespace != "" {
			namespaceToUse = nbCtx.Namespace
		}

		nr := appsv1alpha1.NebulaRestore{
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

		err = client.NebulaRestore().CreateNebulaRestore(&nr)
		if err != nil {
			return ctx, fmt.Errorf("error creating nebula restore [%v/%v]: %v", namespaceToUse, nbCtx.Name, err)
		}

		var stoType, bucketName string
		if nr.Spec.Config.S3 != nil {
			stoType = "S3"
			bucketName = nr.Spec.Config.S3.Bucket
		} else if nr.Spec.Config.GS != nil {
			stoType = "GS"
			bucketName = nr.Spec.Config.GS.Bucket
		}

		return context.WithValue(ctx, nebulaRestoreCtxKey{}, &NebulaRestoreCtxValue{
			Name:        nbCtx.Name,
			Namespace:   namespaceToUse,
			StorageType: stoType,
			BucketName:  bucketName,
		}), nil
	}
}

func GetNebulaRestoreCtxValue(ctx context.Context) *NebulaRestoreCtxValue {
	v := ctx.Value(nebulaRestoreCtxKey{})
	data, _ := v.(*NebulaRestoreCtxValue)
	return data
}

func WaitNebulaRestoreFinished(opts ...NebulaRestoreOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaRestoreOptions{}).WithOptions(opts...)

		restoreContextValue := GetNebulaRestoreCtxValue(ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		var nr *appsv1alpha1.NebulaRestore
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			nr, err = client.NebulaRestore().GetNebulaRestore(restoreContextValue.Namespace, restoreContextValue.Name)
			if err != nil {
				klog.ErrorS(err, "Get NebulaRestore failed", "namespace", restoreContextValue.Namespace, "name", restoreContextValue.Name)
				return false, err
			}
			klog.V(4).InfoS("Waiting for NebulaRestore to complete",
				"namespace", nr.Namespace, "name", nr.Name,
				"generation", nr.Generation, "backup filename", restoreContextValue.BackupFileName,
				"storage type", restoreContextValue.StorageType, "bucket name", restoreContextValue.BucketName,
			)

			if nr.Status.Phase == appsv1alpha1.RestoreComplete {
				return true, nil
			}

			if nr.Status.Phase == appsv1alpha1.RestoreFailed {
				return true, fmt.Errorf("nebula restore [%v/%v] has failed", nr.Namespace, nr.Name)
			}

			return false, nil
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaRestore to complete failed", "namespace", restoreContextValue.Namespace, "name", restoreContextValue.Name)
			return ctx, err
		}

		klog.InfoS("Waiting for NebulaRestore to complete successful", "namespace", restoreContextValue.Namespace, "name", restoreContextValue.Name)

		return context.WithValue(ctx, nebulaRestoreCtxKey{}, &NebulaRestoreCtxValue{
			Name:                    restoreContextValue.Name,
			Namespace:               restoreContextValue.Namespace,
			StorageType:             restoreContextValue.StorageType,
			BucketName:              restoreContextValue.BucketName,
			RestoreClusterNamespace: cfg.Namespace(),
			RestoreClusterName:      nr.Status.ClusterName,
		}), nil
	}
}

func (o *NebulaRestoreOptions) WithOptions(opts ...NebulaRestoreOption) *NebulaRestoreOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithNebulaRestoreWaitOptions(opts ...wait.Option) NebulaRestoreOption {
	return func(o *NebulaRestoreOptions) {
		o.WaitOptions = append(o.WaitOptions, opts...)
	}
}

func DeleteNebulaRestore() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		restoreContextValue := GetNebulaRestoreCtxValue(ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		err = client.NebulaRestore().DeleteNebulaRestore(restoreContextValue.Namespace, restoreContextValue.Name)
		if err != nil {
			return ctx, fmt.Errorf("error deleting nebula restore [%v/%v]: %v", restoreContextValue.Namespace, restoreContextValue.Name, err)
		}

		return ctx, nil
	}
}

func DeleteNebulaRestoredCluster() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		restoreContextValue := GetNebulaRestoreCtxValue(ctx)

		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		err = client.NebulaCluster().DeleteNebulaCluster(restoreContextValue.RestoreClusterNamespace, restoreContextValue.RestoreClusterName)
		if err != nil {
			return ctx, fmt.Errorf("error deleting nebula restore cluster [%v/%v]: %v", restoreContextValue.RestoreClusterNamespace, restoreContextValue.RestoreClusterName, err)
		}

		return ctx, nil
	}
}
