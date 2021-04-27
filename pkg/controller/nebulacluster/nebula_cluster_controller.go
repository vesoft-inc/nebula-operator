/*
Copyright 2021 Vesoft Inc.

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

package nebulacluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component/reclaimer"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	discutil "github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	errorsutil "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const reconcileTimeOut = 10 * time.Second

var ReconcileWaitResult = reconcile.Result{RequeueAfter: reconcileTimeOut}

// ClusterReconciler reconciles a NebulaCluster object
type ClusterReconciler struct {
	Control ControlInterface
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewClusterReconciler(mgr ctrl.Manager) (*ClusterReconciler, error) {
	clientSet, err := kube.NewClientSet(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	sm := component.NewStorageScaler(mgr.GetClient(), clientSet)

	dm, err := discutil.New(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("create discovery client failed: %v", err)
	}
	info, err := dm.GetServerVersion()
	if err != nil {
		return nil, fmt.Errorf("create apiserver info failed: %v", err)
	}

	evenPodsSpread, err := kube.EnableEvenPodsSpread(info)
	if err != nil {
		return nil, fmt.Errorf("get apiServer feature failed: %v", err)
	}

	return &ClusterReconciler{
		Control: NewDefaultNebulaClusterControl(
			clientSet.NebulaCluster(),
			component.NewGraphdCluster(
				clientSet,
				dm,
				evenPodsSpread),
			component.NewMetadCluster(
				clientSet,
				dm,
				evenPodsSpread),
			component.NewStoragedCluster(
				clientSet,
				dm,
				sm,
				evenPodsSpread),
			reclaimer.NewMetaReconciler(clientSet),
			reclaimer.NewPVCReclaimer(clientSet),
			NewClusterConditionUpdater(),
		),
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NebulaCluster"),
		Scheme: mgr.GetScheme(),
	}, nil
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
	var nebulaCluster v1alpha1.NebulaCluster
	subCtx, cancel := context.WithTimeout(ctx, reconcileTimeOut)
	defer cancel()

	key := req.NamespacedName.String()
	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling nebulaCluster %q (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconcile nebulaCluster %q (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile nebulaCluster %q (%v), error: %v", key, time.Since(startTime), retErr)
		}
	}()

	if err := r.Get(subCtx, req.NamespacedName, &nebulaCluster); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("nebula cluster does not exist", "nebulaCluster", klog.KRef(req.Namespace, req.Name))
			if err := component.PvcGc(r.Client, req.Namespace, req.Name); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	klog.InfoS("Start to reconcile ", "nebula cluster", klog.KObj(&nebulaCluster))

	if err := r.syncNebulaCluster(nebulaCluster.DeepCopy()); err != nil {
		if strings.Contains(err.Error(), registry.OptimisticLockErrorMsg) {
			return ReconcileWaitResult, nil
		}

		isReconcileError := func(err error) (b bool) {
			defer func() {
				if b {
					klog.Info(err)
				}
			}()
			return errorsutil.IsReconcileError(err)
		}

		err := errorutils.FilterOut(err, isReconcileError, errorsutil.IsStatusError)
		if err == nil {
			klog.InfoS("nebula cluster need reconcile", "nebulaCluster", klog.KRef(req.Namespace, req.Name))
			return ReconcileWaitResult, nil
		}

		klog.ErrorS(err, "reconcile nebula cluster failed", "nebulaCluster", klog.KRef(req.Namespace, req.Name))
		return ReconcileWaitResult, nil
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) syncNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	return r.Control.UpdateNebulaCluster(nc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaCluster{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&kruisev1alpha1.StatefulSet{}).
		WithOptions(opts).
		Complete(r)
}
