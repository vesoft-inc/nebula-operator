/*
Copyright 2024 Vesoft Inc.

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

package nebulabackup

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	restClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client failed: %v", err)
	}
	eventBroadcaster := record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{QPS: 1})
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedv1.EventSinkImpl{Interface: typedv1.New(kubeClient.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "nebula-backup-controller"})

	backupMgr := NewBackupManager(clientSet, restClient, recorder)

	return &Reconciler{
		control: NewBackupControl(mgr.GetClient(), clientSet, backupMgr),
		client:  mgr.GetClient(),
	}, nil
}

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;list
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulaclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulabackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulabackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.nebula-graph.io,resources=nebulabackups/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
	key := req.NamespacedName.String()
	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.Requeue || res.RequeueAfter > 0 {
				klog.Infof("Finished reconciling NebulaBackup [%s] (%v), result: %v", key, time.Since(startTime), res)
			} else {
				klog.Infof("Finished reconciling NebulaBackup [%s], spendTime: (%v)", key, time.Since(startTime))
			}
		} else {
			klog.Errorf("Failed to reconcile NebulaBackup [%s], spendTime: (%v)", key, time.Since(startTime))
		}
	}()

	var backup v1alpha1.NebulaBackup
	if err := r.client.Get(context.Background(), req.NamespacedName, &backup); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Skipping because NebulaBackup [%s] has been deleted", key)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Info("Start to reconcile NebulaBackup")

	if err := r.syncNebulaBackup(backup.DeepCopy()); err != nil {
		if errorsutil.IsReconcileError(err) {
			klog.Infof("NebulaBackup [%s] reconcile details: %v", key, err)
			return ctrl.Result{RequeueAfter: reconcileTimeOut}, nil
		}
		klog.Errorf("NebulaBackup [%s] reconcile failed: %v", key, err)
		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) syncNebulaBackup(backup *v1alpha1.NebulaBackup) error {
	return r.control.UpdateNebulaBackup(backup)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NebulaBackup{}).
		Complete(r)
}
