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

package nebularestore

import (
	"context"
	"time"

	"github.com/go-logr/logr"
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

// Reconciler reconciles a NebulaRestore object
type Reconciler struct {
	Control ControlInterface
	client.Client
	Log logr.Logger
}

func NewRestoreReconciler(mgr ctrl.Manager) (*Reconciler, error) {
	clientSet, err := kube.NewClientSet(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	restoreMgr := NewRestoreManager(clientSet)

	return &Reconciler{
		Control: NewRestoreControl(clientSet, restoreMgr),
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("NebulaRestore"),
	}, nil
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=restores/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=restores,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
	var restore v1alpha1.NebulaRestore
	key := req.NamespacedName.String()
	subCtx, cancel := context.WithTimeout(ctx, time.Minute*1)
	defer cancel()

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling NebulaRestore [%s] (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconciling NebulaRestore [%s], spendTime: (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile NebulaRestore [%s], spendTime: (%v)", key, time.Since(startTime))
		}
	}()

	if err := r.Get(subCtx, req.NamespacedName, &restore); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Skipping because NebulaRestore [%s] has been deleted", key)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Info("Start to reconcile NebulaRestore")

	if err := r.syncNebulaRestore(restore.DeepCopy()); err != nil {
		if errorsutil.IsReconcileError(err) {
			klog.Infof("NebulaRestore [%s] reconcile details: %v", key, err)
			return ctrl.Result{RequeueAfter: reconcileTimeOut}, nil
		}
		klog.Errorf("NebulaRestore [%s] reconcile failed: %v", key, err)
		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) syncNebulaRestore(restore *v1alpha1.NebulaRestore) error {
	return r.Control.UpdateNebulaRestore(restore)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaRestore{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(r)
}
