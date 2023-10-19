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

package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cbc "k8s.io/component-base/config"

	"github.com/vesoft-inc/nebula-operator/pkg/flag/profile"
	"github.com/vesoft-inc/nebula-operator/pkg/flag/webhook"
)

const (
	NamespaceNebulaSystem = "nebula-system"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	// LeaderElection defines the configuration of leader election client.
	LeaderElection cbc.LeaderElectionConfiguration

	// Period at which the controller forces the repopulation of its local object stores.
	// Defaults to 0, which means the created informer will never do resyncs.
	SyncPeriod metav1.Duration

	// Namespaces restricts the cache's ListWatch to the desired namespaces
	// Default watches all namespaces
	Namespaces []string

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

	// ConcurrentNebulaClusterSyncs is the number of NebulaCluster objects that are
	// allowed to sync concurrently.
	ConcurrentNebulaClusterSyncs int

	// ConcurrentNebulaRestoreSyncs is the number of NebulaRestore objects that are
	// allowed to sync concurrently.
	ConcurrentNebulaRestoreSyncs int

	// EnableAdmissionWebhook enable admission webhook for controller manager.
	EnableAdmissionWebhook bool

	// EnableKruiseScheme enable openkruise scheme for controller manager managing AdvancedStatefulSet.
	EnableKruiseScheme bool

	ProfileOpts profile.Options
	WebhookOpts webhook.Options
}

func NewOptions() *Options {
	return &Options{
		LeaderElection: cbc.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: NamespaceNebulaSystem,
			ResourceName:      "nebula-controller-manager",
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

	flags.DurationVar(&o.SyncPeriod.Duration, "sync-period", 0, "Period at which the controller forces the repopulation of its local object stores.")
	flags.StringSliceVar(&o.Namespaces, "watch-namespaces", nil, "Namespaces restricts the controller watches for updates to Kubernetes objects. If empty, all namespaces are watched. Multiple namespaces seperated by comma.(e.g. ns1,ns2,ns3).")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving.")
	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":8081", "The TCP address that the controller should bind to for serving health probes.(e.g. 127.0.0.1:8081, :8081). It can be set to \"0\" to disable the health probe serving.")
	flags.IntVar(&o.ConcurrentNebulaClusterSyncs, "concurrent-nebulacluster-syncs", 5, "The number of NebulaCluster objects that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentNebulaRestoreSyncs, "concurrent-nebularestore-syncs", 5, "The number of NebulaRestore objects that are allowed to sync concurrently.")
	flags.BoolVar(&o.EnableAdmissionWebhook, "enable-admission-webhook", false, "If set to ture enable admission webhook for controller manager.")
	flags.BoolVar(&o.EnableKruiseScheme, "enable-kruise-scheme", false, "If set to ture enable openkruise scheme for controller manager.")

	o.ProfileOpts.AddFlags(flags)
	o.WebhookOpts.AddFlags(flags)
}
