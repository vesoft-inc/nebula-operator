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
	"flag"
	"net/http"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/scheme"
	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/cmd/autoscaler/app/options"
	certrot "github.com/vesoft-inc/nebula-operator/pkg/cert-rotation"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/autoscaler"
	klogflag "github.com/vesoft-inc/nebula-operator/pkg/flag/klog"
	profileflag "github.com/vesoft-inc/nebula-operator/pkg/flag/profile"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
	nawebhook "github.com/vesoft-inc/nebula-operator/pkg/webhook/autoscaler"
)

// NewAutoscalerCommand creates a *cobra.Command object with default parameters
func NewAutoscalerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "nebula-autoscaler",
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

// Run runs the autoscaler with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("nebula-autoscaler version: %s", version.Version())

	profileflag.ListenAndServe(opts.ProfileOpts)

	if len(opts.Namespaces) == 0 {
		klog.Info("nebula-autoscaler watches all namespaces")
	} else {
		klog.Infof("nebula-autoscaler watches namespaces %v", opts.Namespaces)
	}

	cfg, err := ctrlruntime.GetConfig()
	if err != nil {
		panic(err)
	}

	ctrlOptions := ctrlruntime.Options{
		Scheme:                     scheme.Scheme,
		Logger:                     klog.Background(),
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           opts.LeaderElection.ResourceName,
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaseDuration:              &opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &opts.LeaderElection.RetryPeriod.Duration,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     opts.HealthProbeBindAddress,
		MetricsBindAddress:         opts.MetricsBindAddress,

		Cache: cache.Options{
			SyncPeriod: &opts.HPAOpts.HorizontalPodAutoscalerSyncPeriod.Duration,
			// FIXME: *cache.multiNamespaceInformer missing method GetController
			//Namespaces: opts.Namespaces,
		},
		Controller: config.Controller{
			GroupKindConcurrency: map[string]int{
				v1alpha1.SchemeGroupVersion.WithKind("NebulaAutoscaler").GroupKind().String(): int(opts.HPAOpts.ConcurrentHorizontalPodAutoscalerSyncs),
			},
			RecoverPanic: pointer.Bool(true),
		},
	}

	if opts.EnableAdmissionWebhook {
		ctrlOptions.WebhookServer = webhook.NewServer(webhook.Options{
			Host:          opts.WebhookOpts.BindAddress,
			Port:          opts.WebhookOpts.SecurePort,
			CertDir:       opts.WebhookOpts.CertDir,
			CertName:      opts.WebhookOpts.CertName,
			KeyName:       opts.WebhookOpts.KeyName,
			TLSMinVersion: opts.WebhookOpts.TLSMinVersion,
		})
	}

	mgr, err := ctrlruntime.NewManager(cfg, ctrlOptions)
	if err != nil {
		klog.Errorf("Failed to build nebula-autoscaler: %v", err)
		return err
	}

	controller, err := autoscaler.NewHorizontalController(ctx, mgr,
		opts.HPAOpts.HorizontalPodAutoscalerSyncPeriod.Duration,
		opts.HPAOpts.HorizontalPodAutoscalerDownscaleStabilizationWindow.Duration,
		opts.HPAOpts.HorizontalPodAutoscalerTolerance,
		opts.HPAOpts.HorizontalPodAutoscalerCPUInitializationPeriod.Duration,
		opts.HPAOpts.HorizontalPodAutoscalerInitialReadinessDelay.Duration)
	if err != nil {
		return err
	}
	if err := controller.SetupWithManager(mgr); err != nil {
		klog.Errorf("failed to set up NebulaAutoscaler controller: %v", err)
		return err
	}

	if opts.EnableAdmissionWebhook {
		decoder := admission.NewDecoder(mgr.GetScheme())
		klog.Info("Registering webhooks to nebula-auto-scaler")
		hookServer := mgr.GetWebhookServer()
		hookServer.Register("/validate-nebulaautoscaler",
			&webhook.Admission{Handler: &nawebhook.ValidatingAdmission{Decoder: decoder}})
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
		klog.Errorf("nebula-autoscaler exits unexpectedly: %v", err)
		return err
	}

	return nil
}
