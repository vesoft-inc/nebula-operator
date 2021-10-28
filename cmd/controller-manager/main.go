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

package main

import (
	"flag"
	"os"
	"time"

	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebulacluster"
	"github.com/vesoft-inc/nebula-operator/pkg/logging"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
	"github.com/vesoft-inc/nebula-operator/pkg/webhook"
)

const (
	defaultWatchNamespace                 = corev1.NamespaceAll
	defaultLeaderElectionNamespace        = ""
	defaultLeaderElectionID               = "nebula-controller-manager-leader"
	defaultMetricsBindAddr                = ":8080"
	defaultHealthProbeBindAddr            = ":8081"
	defaultSyncPeriod                     = 30 * time.Minute
	defaultWebhookBindPort                = 9443
	defaultMaxIngressConcurrentReconciles = 3
	defaultPrintVersion                   = false
	defaultEnableLeaderElection           = false
	defaultEnableAdmissionWebhook         = false
)

var (
	scheme = runtime.NewScheme()
	log    = logging.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(kruise.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	var (
		printVersion            bool
		metricsAddr             string
		leaderElectionID        string
		leaderElectionNamespace string
		enableLeaderElection    bool
		enableAdmissionWebhook  bool
		probeAddr               string
		maxConcurrentReconciles int
		watchNamespace          string
		webhookBindPort         int
		syncPeriod              time.Duration
	)

	pflag.BoolVar(&printVersion, "version", defaultPrintVersion, "Show version and quit")
	pflag.StringVar(&metricsAddr, "metrics-bind-address", defaultMetricsBindAddr, "The address the metric endpoint binds to.")
	pflag.StringVar(&probeAddr, "health-probe-bind-address", defaultHealthProbeBindAddr, "The address the probe endpoint binds to.")
	pflag.StringVar(&leaderElectionID, "leader-election-id", defaultLeaderElectionID,
		"Name of the leader election ID to use for this controller")
	pflag.StringVar(&leaderElectionNamespace, "leader-election-namespace", defaultLeaderElectionNamespace,
		"Namespace in which the leader election resource will be created.")
	pflag.BoolVar(&enableLeaderElection, "enable-leader-election", defaultEnableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	pflag.BoolVar(&enableAdmissionWebhook, "admission-webhook", defaultEnableAdmissionWebhook, "Enable admission webhook for controller manager. ")
	pflag.IntVar(&webhookBindPort, "webhook-bind-port", defaultWebhookBindPort,
		"The TCP port the Webhook server binds to.")
	pflag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", defaultMaxIngressConcurrentReconciles, "The max concurrent reconciles.")
	pflag.StringVar(&watchNamespace, "watch-namespace", defaultWatchNamespace,
		"Namespace the controller watches for updates to Kubernetes objects, If empty, all namespaces are watched.")
	pflag.DurationVar(&syncPeriod, "sync-period", defaultSyncPeriod,
		"Period at which the controller forces the repopulation  of its local object stores.")
	opts := logging.Options{
		Development:     true,
		StacktraceLevel: zap.NewAtomicLevelAt(zap.FatalLevel),
		ZapOpts: []zap.Option{
			zap.AddCaller(),
			zap.AddCallerSkip(-1),
		},
	}
	opts.BindFlags(flag.CommandLine)

	pflag.Parse()
	logging.SetLogger(logging.New(logging.UseFlagOptions(&opts)))

	if printVersion {
		log.Info("Nebula Operator Version", "version", version.Version())
		os.Exit(0)
	}

	log.Info("Welcome to Nebula Operator.")
	log.Info("Nebula Operator Version", "version", version.Version())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       webhookBindPort,
		HealthProbeBindAddress:     probeAddr,
		LeaderElection:             enableLeaderElection,
		LeaderElectionResourceLock: resourcelock.ConfigMapsLeasesResourceLock,
		LeaderElectionNamespace:    leaderElectionNamespace,
		LeaderElectionID:           leaderElectionID,
		Namespace:                  watchNamespace,
		SyncPeriod:                 &syncPeriod,
	})
	if err != nil {
		log.Error(err, "unable to start controller-manager")
		os.Exit(1)
	}

	nebulaClusterReconciler, err := nebulacluster.NewClusterReconciler(mgr)
	if err != nil {
		log.Error(err, "unable to create nebula cluster reconciler", "controller", "NebulaCluster")
		os.Exit(1)
	}

	if err := nebulaClusterReconciler.SetupWithManager(mgr,
		controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		log.Error(err, "unable to create controller", "controller", "NebulaCluster")
		os.Exit(1)
	}

	if enableAdmissionWebhook {
		log.Info("setup webhook")
		if err = webhook.SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to setup webhook")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
