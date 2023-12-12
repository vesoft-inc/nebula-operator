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
	"time"

	kruisev1beta1 "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component/reclaimer"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	discutil "github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
	errorsutil "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

const (
	defaultTimeout   = 5 * time.Second
	reconcileTimeOut = 10 * time.Second

	KruiseReferenceName = "statefulsets.apps.kruise.io"
)

// ClusterReconciler reconciles a NebulaCluster object
type ClusterReconciler struct {
	control      ControlInterface
	client       client.Client
	enableKruise bool
}

func NewClusterReconciler(mgr ctrl.Manager, enableKruise bool) (*ClusterReconciler, error) {
	clientSet, err := kube.NewClientSet(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	sm := component.NewStorageScaler(clientSet)
	graphdUpdater := component.NewGraphdUpdater(clientSet.Pod())
	metadUpdater := component.NewMetadUpdater(clientSet.Pod())
	storagedUpdater := component.NewStoragedUpdater(clientSet)
	storagedFailover := component.NewStoragedFailover(mgr.GetClient(), clientSet)

	dm, err := discutil.New(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("create discovery client failed: %v", err)
	}
	info, err := dm.GetServerVersion()
	if err != nil {
		return nil, fmt.Errorf("create apiserver info failed: %v", err)
	}

	valid, err := kube.ValidVersion(info)
	if err != nil {
		return nil, fmt.Errorf("get server version failed: %v", err)
	}
	if !valid {
		return nil, fmt.Errorf("server version not supported")
	}

	return &ClusterReconciler{
		control: NewDefaultNebulaClusterControl(
			mgr.GetClient(),
			clientSet.NebulaCluster(),
			component.NewGraphdCluster(
				clientSet,
				dm,
				graphdUpdater),
			component.NewMetadCluster(
				clientSet,
				dm,
				metadUpdater),
			component.NewStoragedCluster(
				clientSet,
				dm,
				sm,
				storagedUpdater,
				storagedFailover),
			component.NewNebulaExporter(clientSet),
			component.NewNebulaConsole(clientSet),
			reclaimer.NewMetaReconciler(clientSet),
			reclaimer.NewPVCReclaimer(clientSet),
			NewClusterConditionUpdater(),
		),
		client:       mgr.GetClient(),
		enableKruise: enableKruise,
	}, nil
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
	key := req.NamespacedName.String()
	subCtx, cancel := context.WithTimeout(ctx, time.Minute*1)
	defer cancel()

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling NebulaCluster [%s] (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconciling NebulaCluster [%s], spendTime: (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile NebulaCluster [%s], spendTime: (%v)", key, time.Since(startTime))
		}
	}()

	var nebulaCluster v1alpha1.NebulaCluster
	if err := r.client.Get(subCtx, req.NamespacedName, &nebulaCluster); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Skipping because NebulaCluster [%s] has been deleted", key)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Info("Start to reconcile NebulaCluster")

	if !r.enableKruise && nebulaCluster.Spec.Reference.Name == KruiseReferenceName {
		return ctrl.Result{}, fmt.Errorf("openkruise scheme not registered")
	}

	if err := r.syncNebulaCluster(nebulaCluster.DeepCopy()); err != nil {
		isReconcileError := func(err error) (b bool) {
			defer func() {
				if b {
					klog.Infof("NebulaCluster [%s] reconcile details: %v", key, err)
				}
			}()
			return errorsutil.IsReconcileError(err)
		}

		err := errorutils.FilterOut(err, isReconcileError, errorsutil.IsDNSError, errorsutil.IsStatusError)
		if err == nil {
			return ctrl.Result{RequeueAfter: reconcileTimeOut}, nil
		}

		klog.Errorf("NebulaCluster [%s] reconcile failed: %v", key, err)

		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) syncNebulaCluster(nc *v1alpha1.NebulaCluster) error {
	if nc.DeletionTimestamp != nil {
		return r.control.DeleteCluster(nc)
	}
	return r.control.UpdateNebulaCluster(nc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.enableKruise {
		return ctrl.NewControllerManagedBy(mgr).
			For(&v1alpha1.NebulaCluster{}).
			Owns(&corev1.ConfigMap{}).
			Owns(&corev1.Service{}).
			Owns(&appsv1.StatefulSet{}).
			Owns(&kruisev1beta1.StatefulSet{}).
			Owns(&appsv1.Deployment{}).
			Complete(r)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaCluster{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
