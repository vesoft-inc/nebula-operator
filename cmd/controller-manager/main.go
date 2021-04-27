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
	"fmt"
	"os"

	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	runtimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebulacluster"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
	"github.com/vesoft-inc/nebula-operator/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
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
		enableLeaderElection    bool
		enableAdmissionWebhook  bool
		probeAddr               string
		maxConcurrentReconciles int
	)

	pflag.BoolVar(&printVersion, "version", false, "Show version and quit")
	pflag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	pflag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	pflag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	pflag.BoolVar(&enableAdmissionWebhook, "admission-webhook", false, "Enable admission webhook for controller manager. ")
	pflag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 2, "The max concurrent reconciles.")
	opts := runtimezap.Options{
		Development:     true,
		StacktraceLevel: zap.NewAtomicLevelAt(zap.FatalLevel),
		ZapOpts: []zap.Option{
			zap.AddCaller(),
			zap.AddCallerSkip(-1),
		},
	}
	opts.BindFlags(flag.CommandLine)

	pflag.Parse()

	if printVersion {
		fmt.Printf("Nebula Operator Version: %#v\n", version.Version())
		os.Exit(0)
	}
	klog.Info("Welcome to Nebula Operator.")
	klog.Infof("Nebula Operator Version: %#v", version.Version())

	ctrl.SetLogger(runtimezap.New(runtimezap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "467c28e9.nebula-graph.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	nebulaClusterReconciler, err := nebulacluster.NewClusterReconciler(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create nebula cluster reconciler", "controller", "NebulaCluster")
		os.Exit(1)
	}

	if err := nebulaClusterReconciler.SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NebulaCluster")
		os.Exit(1)
	}

	if enableAdmissionWebhook {
		setupLog.Info("setup webhook")
		if err = webhook.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to setup webhook")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
