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

package app

import (
	"context"
	"crypto/tls"
	"flag"
	"net/http"

	kruisev1beta1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/cmd/controller-manager/app/options"
	certrot "github.com/vesoft-inc/nebula-operator/pkg/cert-rotation"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/cronbackup"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebulabackup"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebulacluster"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/nebularestore"
	klogflag "github.com/vesoft-inc/nebula-operator/pkg/flag/klog"
	profileflag "github.com/vesoft-inc/nebula-operator/pkg/flag/profile"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
	ncwebhook "github.com/vesoft-inc/nebula-operator/pkg/webhook/nebulacluster"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "nebula-controller-manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(ctx, opts)
		},
	}

	nfs := cliflag.NamedFlagSets{}
	fs := nfs.FlagSet("generic")
	fs.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(fs)

	logsFlagSet := nfs.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(fs)
	cmd.Flags().AddFlagSet(logsFlagSet)

	return cmd
}

// Run runs the controller-manager with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("nebula-controller-manager version: %s", version.Version())

	logf.SetLogger(klog.Background())

	profileflag.ListenAndServe(opts.ProfileOpts)

	if opts.EnableKruiseScheme {
		utilruntime.Must(kruisev1beta1.AddToScheme(clientgoscheme.Scheme))
		utilruntime.Must(kruisev1beta1.AddToScheme(scheme))
		klog.Info("register openkruise scheme")
	}

	if len(opts.Namespaces) == 0 {
		klog.Info("nebula-controller-manager watches all namespaces")
	} else {
		klog.Infof("nebula-controller-manager watches namespaces %v", opts.Namespaces)
	}

	cfg, err := ctrlruntime.GetConfig()
	if err != nil {
		panic(err)
	}

	watchedNamespaces := make(map[string]cache.Config)
	for _, ns := range opts.Namespaces {
		watchedNamespaces[ns] = cache.Config{}
	}

	ctrlOptions := ctrlruntime.Options{
		Scheme:                     scheme,
		Logger:                     klog.Background(),
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           opts.LeaderElection.ResourceName,
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaseDuration:              &opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &opts.LeaderElection.RetryPeriod.Duration,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     opts.HealthProbeBindAddress,
		Metrics: metricsserver.Options{
			BindAddress: opts.MetricsBindAddress,
		},
		Cache: cache.Options{
			SyncPeriod:        &opts.SyncPeriod.Duration,
			DefaultNamespaces: watchedNamespaces,
		},
		Controller: config.Controller{
			GroupKindConcurrency: map[string]int{
				v1alpha1.SchemeGroupVersion.WithKind("NebulaCluster").GroupKind().String():    opts.ConcurrentNebulaClusterSyncs,
				v1alpha1.SchemeGroupVersion.WithKind("NebulaBackup").GroupKind().String():     opts.ConcurrentNebulaBackupSyncs,
				v1alpha1.SchemeGroupVersion.WithKind("NebulaCronBackup").GroupKind().String(): opts.ConcurrentNebulaCronBackupSyncs,
				v1alpha1.SchemeGroupVersion.WithKind("NebulaRestore").GroupKind().String():    opts.ConcurrentNebulaRestoreSyncs,
			},
			RecoverPanic: pointer.Bool(true),
		},
	}
	if opts.EnableAdmissionWebhook {
		ctrlOptions.WebhookServer = webhook.NewServer(webhook.Options{
			Host:     opts.WebhookOpts.BindAddress,
			Port:     opts.WebhookOpts.SecurePort,
			CertDir:  opts.WebhookOpts.CertDir,
			CertName: opts.WebhookOpts.CertName,
			KeyName:  opts.WebhookOpts.KeyName,
			TLSOpts: []func(*tls.Config){
				func(config *tls.Config) {
					switch opts.WebhookOpts.TLSMinVersion {
					case "1.0":
						config.MinVersion = tls.VersionTLS10
					case "1.1":
						config.MinVersion = tls.VersionTLS11
					case "1.2":
						config.MinVersion = tls.VersionTLS12
					case "1.3":
						config.MinVersion = tls.VersionTLS13
					}
				},
			},
		})
	}

	if opts.NebulaSelector != "" {
		parsedSelector, err := labels.Parse(opts.NebulaSelector)
		if err != nil {
			klog.Errorf("couldn't convert selector into a corresponding internal selector object: %v", err)
			return err
		}
		ctrlOptions.Cache.DefaultLabelSelector = parsedSelector
	}

	mgr, err := ctrlruntime.NewManager(cfg, ctrlOptions)
	if err != nil {
		klog.Errorf("Failed to build controller manager: %v", err)
		return err
	}

	clusterReconciler, err := nebulacluster.NewClusterReconciler(mgr, opts.EnableKruiseScheme)
	if err != nil {
		return err
	}
	if err := clusterReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("failed to set up NebulaCluster controller: %v", err)
		return err
	}

	backupReconciler, err := nebulabackup.NewBackupReconciler(mgr)
	if err != nil {
		return err
	}
	if err := backupReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("failed to set up NebulaBackup controller: %v", err)
		return err
	}

	cronBackupReconciler, err := cronbackup.NewCronBackupReconciler(mgr)
	if err != nil {
		return err
	}
	if err := cronBackupReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("failed to set up NebulaCronBackup controller: %v", err)
		return err
	}

	restoreReconciler, err := nebularestore.NewRestoreReconciler(mgr)
	if err != nil {
		return err
	}
	if err := restoreReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("failed to set up NebulaRestore controller: %v", err)
		return err
	}

	if opts.EnableAdmissionWebhook {
		decoder := admission.NewDecoder(mgr.GetScheme())
		klog.Info("Registering webhooks to nebula-controller-manager")
		hookServer := mgr.GetWebhookServer()
		hookServer.Register("/validate-nebulacluster",
			&webhook.Admission{Handler: &ncwebhook.ValidatingAdmission{Decoder: decoder}})
		hookServer.WebhookMux().Handle("/readyz/", http.StripPrefix("/readyz/", &healthz.Handler{}))

		// Start certificate rotation
		certGenerator := certrot.CertGenerator{
			LeaderElection:    opts.LeaderElection,
			WebhookNames:      opts.WebhookOpts.WebhookNames,
			WebhookServerName: opts.WebhookOpts.WebhookServerName,
			WebhookNamespace:  opts.WebhookOpts.WebhookNamespace,
			CertDir:           opts.WebhookOpts.CertDir,
			CertValidity:      opts.WebhookOpts.CertValidity,
			SecretName:        opts.WebhookOpts.SecretName,
			SecretNamespace:   opts.WebhookOpts.SecretNamespace,
			KubernetesDomain:  opts.WebhookOpts.KubernetesDomain,
		}

		if opts.WebhookOpts.UseCertGenerator {
			err = certGenerator.Run(ctx)
			if err != nil {
				klog.Errorf("failed to generate certificate for webhook. err: %v", err)
				return err
			}
		} else {
			klog.Info("certificate generation disabled. will not generate self signed certificate")
		}
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("failed to add health check endpoint: %v", err)
		return err
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Errorf("failed to add ready check endpoint: %v", err)
		return err
	}

	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("nebula-controller-manager exits unexpectedly: %v", err)
		return err
	}

	return nil
}
