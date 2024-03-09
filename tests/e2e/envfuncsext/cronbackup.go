package envfuncsext

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeployNebulaCronBackup(incremental bool, nbCtx NebulaBackupInstallOptions) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		namespaceToUse := cfg.Namespace()
		if nbCtx.Namespace != "" {
			namespaceToUse = nbCtx.Namespace
		}

		disable := false
		ncb := &appsv1alpha1.NebulaCronBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nbCtx.Name,
				Namespace: namespaceToUse,
			},
			Spec: appsv1alpha1.CronBackupSpec{
				Schedule:       nbCtx.CronBackupOps.Schedule,
				Pause:          &disable,
				BackupTemplate: nbCtx.Spec,
			},
		}

		ctx, err := CreateObject(ncb)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("error creating nebula cron backup [%v/%v]: %v", namespaceToUse, nbCtx.Name, err)
		}

		key := nebulaBackupCtxKey{backupType: "base"}
		if incremental {
			key = nebulaBackupCtxKey{backupType: "incr"}
		}

		var stoType, region, bucketName string
		if ncb.Spec.BackupTemplate.Config.S3 != nil {
			stoType = "S3"
			region = ncb.Spec.BackupTemplate.Config.S3.Region
			bucketName = ncb.Spec.BackupTemplate.Config.S3.Bucket
		} else if ncb.Spec.BackupTemplate.Config.GS != nil {
			stoType = "GS"
			region = ncb.Spec.BackupTemplate.Config.GS.Location
			bucketName = ncb.Spec.BackupTemplate.Config.GS.Bucket
		}

		return context.WithValue(ctx, key, &NebulaBackupCtxValue{
			Name:            nbCtx.Name,
			Namespace:       namespaceToUse,
			StorageType:     stoType,
			Region:          region,
			BucketName:      bucketName,
			CleanBackupData: *ncb.Spec.BackupTemplate.CleanBackupData,
			Schedule:        nbCtx.CronBackupOps.Schedule,
			TestPause:       nbCtx.CronBackupOps.TestPause,
			BackupSpec:      *nbCtx.Spec.DeepCopy(),
		}), nil
	}
}

func WaitNebulaCronBackupFinished(incremental bool, opts ...NebulaBackupOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaBackupOptions{}).WithOptions(opts...)

		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		ncb := &appsv1alpha1.NebulaCronBackup{}
		nb := &appsv1alpha1.NebulaBackup{}
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			err = cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, ncb)
			if err != nil {
				klog.ErrorS(err, "Get NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return true, err
			}

			if pointer.BoolDeref(ncb.Spec.Pause, false) {
				err = fmt.Errorf("NebulaCronBackup is still paused")
				klog.ErrorS(err, "check NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return true, err
			}

			if ncb.Status.LastBackup == "" || backupContextValue.BackupFileName == ncb.Status.LastBackup {
				klog.V(4).InfoS("Waiting for NebulaCronBackup to trigger backup",
					"namespace", ncb.Namespace, "name", ncb.Name,
					"generation", ncb.Generation,
				)
				return false, nil
			}

			err = cfg.Client().Resources().Get(ctx, ncb.Status.LastBackup, backupContextValue.Namespace, nb)
			if err != nil {
				klog.ErrorS(err, "Get NebulaBackup failed", "namespace", backupContextValue.Namespace, "name", ncb.Status.LastBackup)
				return true, err
			}

			if nb.Status.Phase == appsv1alpha1.BackupComplete {
				return true, nil
			}

			if nb.Status.Phase == appsv1alpha1.BackupFailed {
				return true, fmt.Errorf("nebula backup [%v/%v] has failed", nb.Namespace, nb.Name)
			}

			klog.V(4).InfoS("Waiting for backup triggered by NebulaCronBackup to complete",
				"namespace", ncb.Namespace, "name", ncb.Name,
				"generation", ncb.Generation, "triggered backup name", ncb.Status.LastBackup,
			)

			return false, nil
		}, o.WaitOptions...); err != nil {
			if ncb.Status.LastBackup == "" {
				klog.ErrorS(err, "Waiting for NebulaCronBackup to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
			} else {
				klog.ErrorS(err, "Waiting for NebulaBackup triggered by NebulaCronBackup to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "triggered backup name", ncb.Status.LastBackup)
			}
			return ctx, err
		}

		klog.InfoS("Waiting for NebulaBackup triggered by NebulaCronBackup to complete successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "triggered backup name", nb.Name, "backup file name", nb.Status.BackupName)

		key := nebulaBackupCtxKey{backupType: "base"}
		if incremental {
			key = nebulaBackupCtxKey{backupType: "incr"}
		}

		return context.WithValue(ctx, key, &NebulaBackupCtxValue{
			Name:                backupContextValue.Name,
			Namespace:           backupContextValue.Namespace,
			BackupFileName:      nb.Status.BackupName,
			StorageType:         backupContextValue.StorageType,
			Region:              backupContextValue.Region,
			BucketName:          backupContextValue.BucketName,
			CleanBackupData:     backupContextValue.CleanBackupData,
			Schedule:            backupContextValue.Schedule,
			TestPause:           backupContextValue.TestPause,
			TriggeredBackupName: nb.Name,
			BackupSpec:          backupContextValue.BackupSpec,
		}), nil
	}
}

func SetCronBackupPause(incremental, pause bool) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		ncb := &appsv1alpha1.NebulaCronBackup{}
		err := cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, ncb)
		if err != nil {
			klog.ErrorS(err, "Get NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
			return ctx, err
		}

		ncb.Spec.Pause = &pause

		err = cfg.Client().Resources().Update(ctx, ncb)
		if err != nil {
			if pause {
				return ctx, fmt.Errorf("error pausing nebula cron backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
			} else {
				return ctx, fmt.Errorf("error resuming nebula cron backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
			}
		}

		return ctx, nil
	}
}

func CheckCronBackupPaused(incremental bool, opts ...NebulaBackupOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaBackupOptions{}).WithOptions(opts...)

		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		ncb := &appsv1alpha1.NebulaCronBackup{}
		firstTime := true
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			err = cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, ncb)
			if err != nil {
				klog.ErrorS(err, "Get NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return true, err
			}

			if !pointer.BoolDeref(ncb.Spec.Pause, false) {
				err = fmt.Errorf("nebula cron backup is not paused")
				klog.ErrorS(err, "Pausing NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
				return false, err
			}

			if !firstTime {
				if ncb.Status.LastBackup != backupContextValue.TriggeredBackupName {
					err = fmt.Errorf("nubula cron backup was not paused successfully. New backup was triggered. Backup name %v does not match previous backup name %v", ncb.Status.LastBackup, backupContextValue.TriggeredBackupName)
					klog.ErrorS(err, "Pausing NebulaCronBackup failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
					return true, err
				}
				return true, nil
			} else {
				klog.V(4).Infof("NebulaCronBackup [%v/%v] was just paused. Will check if pause was successful during the next duration.", backupContextValue.Namespace, backupContextValue.Name)
				firstTime = false
				return false, nil
			}
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaCronBackup to pause failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
			return ctx, err
		}

		klog.InfoS("Waiting for NebulaCronBackup to pause successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name)
		return ctx, nil
	}
}

func DeleteNebulaCronBackup(incremental bool) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		ncb := &appsv1alpha1.NebulaCronBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupContextValue.Name,
				Namespace: backupContextValue.Namespace,
			},
		}
		ctx, err := DeleteObject(ncb)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("error deleting nebula cron backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
		}

		return ctx, nil
	}
}

func WaitForCleanCronBackup(incremental bool, opts ...NebulaBackupOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := (&NebulaBackupOptions{}).WithOptions(opts...)

		backupContextValue := GetNebulaBackupCtxValue(incremental, ctx)

		ncb := &appsv1alpha1.NebulaCronBackup{}
		if err := wait.For(func(ctx context.Context) (done bool, err error) {
			err = cfg.Client().Resources().Get(ctx, backupContextValue.Name, backupContextValue.Namespace, ncb)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return true, fmt.Errorf("error deleting nebula cron backup [%v/%v]: %v", backupContextValue.Namespace, backupContextValue.Name, err)
			}

			klog.V(4).InfoS("Waiting for NebulaCronBackup cleanup to complete",
				"namespace", ncb.Namespace, "name", ncb.Name, "file name", backupContextValue.BackupFileName,
				"generation", ncb.Generation,
			)
			return false, nil

		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for NebulaCronBackup clean to complete failed", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)
			return ctx, err
		}
		klog.InfoS("Waiting for NebulaCronBackup clean to complete successful", "namespace", backupContextValue.Namespace, "name", backupContextValue.Name, "file name", backupContextValue.BackupFileName)

		return ctx, nil
	}
}
