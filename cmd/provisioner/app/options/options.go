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
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/v9/controller"
)

const (
	defaultProvisioner    = "nebula-cloud.io/local-pv"
	defaultHelperImage    = "vesoft/nebula-alpine:latest"
	defaultServiceAccount = "local-pv-provisioner-sa"
	defaultConfigMap      = "local-pv-config"

	defaultResyncPeriod             = 5 * time.Minute
	defaultThreadiness              = pvController.DefaultThreadiness
	defaultFailedProvisionThreshold = pvController.DefaultFailedProvisionThreshold
	defaultFailedDeleteThreshold    = pvController.DefaultFailedDeleteThreshold
)

type Options struct {
	// leaderElect enables a leader election client to gain leadership
	// before executing the main loop
	LeaderElect bool

	// The name of the provisioner for which this controller dynamically
	// provisions volumes
	Provisioner string

	// The helper image used for managing mount point on the host
	HelperImage string

	// Image pull secret used for pulling helper image
	ImagePullSecret string

	// The name of the provisioner service account
	ServiceAccount string

	// The name of the provisioner configmap
	ConfigMap string

	// The number of claim and volume workers each to launch
	Threadiness int

	// The threshold for max number of retries on failures of volume Provision
	FailedProvisionThreshold int

	// The threshold for max number of retries on failures of volume Delete
	FailedDeleteThreshold int

	// Period at which the controller forces the repopulation of its local object stores
	ResyncPeriod metav1.Duration
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.LeaderElect, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.Provisioner, "provisioner-name", defaultProvisioner, "The name of the provisioner for which this controller dynamically provisions volumes.")
	flags.StringVar(&o.HelperImage, "helper-image", defaultHelperImage, "The helper image used for managing mount point on the host.")
	flags.StringVar(&o.ImagePullSecret, "image-pull-secret", "", "Image pull secret used for pulling helper image.")
	flags.StringVar(&o.ServiceAccount, "service-account", defaultServiceAccount, "The name of the provisioner service account.")
	flags.StringVar(&o.ConfigMap, "configmap", defaultConfigMap, "The name of the provisioner configmap.")
	flags.IntVar(&o.Threadiness, "threadiness", defaultThreadiness, "The number of claim and volume workers each to launch.")
	flags.IntVar(&o.FailedProvisionThreshold, "failed-provision-threshold", defaultFailedProvisionThreshold, "The threshold for max number of retries on failures of volume Provision.")
	flags.IntVar(&o.FailedDeleteThreshold, "failed-delete-threshold", defaultFailedDeleteThreshold, "The threshold for max number of retries on failures of volume Delete.")
	flags.DurationVar(&o.ResyncPeriod.Duration, "resync-period", defaultResyncPeriod, "Period at which the controller forces the repopulation of its local object stores.")
}
