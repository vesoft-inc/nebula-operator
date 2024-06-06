/*
Copyright 2024 Vesoft Inc.
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
)

const (
	NamespaceNebulaSystem = "nebula-system"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

type Options struct {
	// LeaderElection defines the configuration of leader election client.
	LeaderElection cbc.LeaderElectionConfiguration

	// Webhook name represents the name of the webhook server associated with the certificate.
	WebhookName string

	// Webhook namespace represents the namespace of the webhook server associated with the certificate.
	WebhookNamespace string

	// CertDir represents the directory to save the certificates in
	CertDir string

	// CertValidity represents the number of days the certificate should be valid for
	CertValidity int64
}

func NewOptions() *Options {
	return &Options{
		LeaderElection: cbc.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: NamespaceNebulaSystem,
			ResourceName:      "nebula-certificate-generator",
		},
	}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
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
	flags.StringVar(&o.WebhookName, "webhook-name", "nebulaWebhook", "Specifies the name of the webhook to associate with the certificate")
	flags.StringVar(&o.WebhookNamespace, "webhook-namespace", "default", "Specifies the namespace of the webhook to associate with the certificate")
	flags.StringVar(&o.CertDir, "certificate-dir", "/etc/cert", "Specifies the directory in which to save the generated webhook certificates")
	flags.Int64Var(&o.CertValidity, "certificate-validity", 365, "Specifies the number of days the certificate should be valid for")
}
