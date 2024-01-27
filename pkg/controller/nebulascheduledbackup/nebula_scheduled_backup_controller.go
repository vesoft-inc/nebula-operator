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

package nebulascheduledbackup

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	errorsutil "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	defaultTimeout   = 5 * time.Second
	reconcileTimeOut = 10 * time.Second
)

var _ reconcile.Reconciler = (*Reconciler)(nil)

// Reconciler reconciles a NebulaBackup object
type Reconciler struct {
	control ControlInterface
	client  client.Client
}

func NewBackupReconciler(mgr ctrl.Manager) (*Reconciler, error) {
	clientSet, err := kube.NewClientSet(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	backupMgr := NewBackupManager(clientSet)

	return &Reconciler{
		control: NewBackupControl(clientSet, backupMgr),
		client:  mgr.GetClient(),
	}, nil
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulascheduledbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulascheduledbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulascheduledbackups/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
	key := req.NamespacedName.String()
	subCtx, cancel := context.WithTimeout(ctx, time.Minute*1)
	defer cancel()

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling NebulaScheduledBackup [%s] (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconciling NebulaScheduledBackup [%s], spendTime: (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile NebulaScheduledBackup [%s], spendTime: (%v)", key, time.Since(startTime))
		}
	}()

	var scheduledBackup v1alpha1.NebulaScheduledBackup
	if err := r.client.Get(subCtx, req.NamespacedName, &scheduledBackup); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Skipping because NebulaScheduledBackup [%s] has been deleted", key)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Info("Start to reconcile NebulaScheduledBackup")

	// Check the current resource version
	currentResourceVersion := scheduledBackup.GetResourceVersion()
	klog.Infof("Current resource version: %v", currentResourceVersion)

	updatedScheduledBackup := scheduledBackup.DeepCopy()
	newReconcilerDuration, err := r.syncNebulaScheduledBackup(updatedScheduledBackup)
	if err != nil {
		if errorsutil.IsReconcileError(err) {
			klog.Infof("NebulaScheduledBackup [%s] reconcile details: %v", key, err)
			return ctrl.Result{RequeueAfter: reconcileTimeOut}, err
		}
		klog.Errorf("NebulaScheduledBackup [%s] reconcile failed: %v", key, err)
		return ctrl.Result{RequeueAfter: defaultTimeout}, err
	}

	if newReconcilerDuration != nil {
		// wait until next backup time to reconcile to avoid wasting resources.
		return ctrl.Result{Requeue: true, RequeueAfter: *newReconcilerDuration}, nil
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) syncNebulaScheduledBackup(backup *v1alpha1.NebulaScheduledBackup) (*time.Duration, error) {
	newReconcilerDuration, err := r.control.Sync(backup)
	return newReconcilerDuration, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaScheduledBackup{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(r)
}
