/*
Copyright 2023 Vesoft Inc.
Copyright 2015 The Kubernetes Authors.

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

package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cbc "k8s.io/component-base/config"
	ctrlmgrconfigv1alpha1 "k8s.io/kube-controller-manager/config/v1alpha1"

	"github.com/vesoft-inc/nebula-operator/pkg/flag/profile"
)

const (
	NamespaceNebulaSystem = "nebula-system"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}

	defaultAutoscalerSyncPeriod                   = metav1.Duration{Duration: 15 * time.Second}
	defaultAutoscalerUpscaleForbiddenWindow       = metav1.Duration{Duration: 3 * time.Minute}
	defaultAutoscalerDownscaleStabilizationWindow = metav1.Duration{Duration: 5 * time.Minute}
	defaultAutoscalerCPUInitializationPeriod      = metav1.Duration{Duration: 5 * time.Minute}
	defaultAutoscalerInitialReadinessDelay        = metav1.Duration{Duration: 30 * time.Second}
	defaultAutoscalerDownscaleForbiddenWindow     = metav1.Duration{Duration: 5 * time.Minute}
)

type Options struct {
	// Namespaces restricts the cache's ListWatch to the desired namespaces
	// Default watches all namespaces
	Namespaces []string

	// LeaderElection defines the configuration of leader election client.
	LeaderElection cbc.LeaderElectionConfiguration

	// HPAOpts defines the configuration of autoscaler controller.
	HPAOpts ctrlmgrconfigv1alpha1.HPAControllerConfiguration

	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string

	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes.
	//	// It can be set to "0" to disable serving the health probe.
	// Defaults to ":8081".
	HealthProbeBindAddress string

	ProfileOpts profile.Options
}

func NewOptions() *Options {
	return &Options{
		LeaderElection: cbc.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: NamespaceNebulaSystem,
			ResourceName:      "nebula-autoscaler",
		},
	}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", NamespaceNebulaSystem, "The namespace of resource object that is used for locking during leader election.")
	flags.DurationVar(&o.LeaderElection.LeaseDuration.Duration, "leader-elect-lease-duration", defaultElectionLeaseDuration.Duration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	flags.DurationVar(&o.LeaderElection.RenewDeadline.Duration, "leader-elect-renew-deadline", defaultElectionRenewDeadline.Duration, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	flags.DurationVar(&o.LeaderElection.RetryPeriod.Duration, "leader-elect-retry-period", defaultElectionRetryPeriod.Duration, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")

	flags.Int32Var(&o.HPAOpts.ConcurrentHorizontalPodAutoscalerSyncs, "concurrent-autoscaler-syncs", 5, "The number of nebula autoscaler objects that are allowed to sync concurrently. Larger number = more responsive nebula autoscaler objects processing, but more CPU (and network) load.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerSyncPeriod.Duration, "autoscaler-sync-period", defaultAutoscalerSyncPeriod.Duration, "The period for syncing the number of pods in nebula autoscaler.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerUpscaleForbiddenWindow.Duration, "autoscaler-upscale-delay", defaultAutoscalerUpscaleForbiddenWindow.Duration, "The period since last upscale, before another upscale can be performed in nebula autoscaler.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerDownscaleStabilizationWindow.Duration, "autoscaler-downscale-stabilization", defaultAutoscalerDownscaleStabilizationWindow.Duration, "The period for which nebula autoscaler will look backwards and not scale down below any recommendation it made during that period.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerDownscaleForbiddenWindow.Duration, "autoscaler-downscale-delay", defaultAutoscalerDownscaleForbiddenWindow.Duration, "The period since last downscale, before another downscale can be performed in nebula autoscaler.")
	flags.Float64Var(&o.HPAOpts.HorizontalPodAutoscalerTolerance, "autoscaler-tolerance", 0.1, "The minimum change (from 1.0) in the desired-to-actual metrics ratio for the nebula autoscaler to consider scaling.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerCPUInitializationPeriod.Duration, "autoscaler-cpu-initialization-period", defaultAutoscalerCPUInitializationPeriod.Duration, "The period after pod start when CPU samples might be skipped.")
	flags.DurationVar(&o.HPAOpts.HorizontalPodAutoscalerInitialReadinessDelay.Duration, "autoscaler-initial-readiness-delay", defaultAutoscalerInitialReadinessDelay.Duration, "The period after pod start during which readiness changes will be treated as initial readiness.")

	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving.")
	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":8081", "The TCP address that the controller should bind to for serving health probes.(e.g. 127.0.0.1:8081, :8081). It can be set to \"0\" to disable the health probe serving.")

	//flags.StringSliceVar(&o.Namespaces, "watch-namespaces", nil, "Namespaces restricts the controller watches for updates to Kubernetes objects. If empty, all namespaces are watched. Multiple namespaces seperated by comma.(e.g. ns1,ns2,ns3).")
}
