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
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/v9/controller"

	"github.com/vesoft-inc/nebula-operator/cmd/provisioner/app/options"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/provisioner"
	klogflag "github.com/vesoft-inc/nebula-operator/pkg/flag/klog"
)

const (
	defaultProvisionerNamespace = "local-pv-provisioner"
)

// NewProvisionerCommand creates a *cobra.Command object with default parameters
func NewProvisionerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "local-pv-provisioner",
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

// Run runs the provisioner with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	c, err := config.GetConfig()
	if err != nil {
		return err
	}
	kubeClient, err := clientset.NewForConfig(c)
	if err != nil {
		return err
	}

	cacheConfig := &provisioner.CacheConfig{
		ProvisionerName: opts.Provisioner,
		Cache:           provisioner.NewVolumeCache(),
		InformerFactory: informers.NewSharedInformerFactory(kubeClient, time.Second*5),
	}
	provisioner.NewPopulator(cacheConfig)

	// Start informers after all event listeners are registered.
	cacheConfig.InformerFactory.Start(ctx.Done())
	// Wait for all started informers' cache were synced.
	for v, synced := range cacheConfig.InformerFactory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			klog.Fatalf("Error syncing informer for %v", v)
		}
	}

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = defaultProvisionerNamespace
	}
	p := provisioner.NewProvisioner(ctx, kubeClient, namespace, opts.HelperImage, opts.ConfigMap, opts.ServiceAccount)
	p.CacheConfig = cacheConfig
	if opts.ImagePullSecret != "" {
		p.SetImagePullSecret(opts.ImagePullSecret)
	}
	pc := pvController.NewProvisionController(
		kubeClient,
		opts.Provisioner,
		p,
		pvController.LeaderElection(opts.LeaderElect),
		pvController.FailedProvisionThreshold(opts.FailedProvisionThreshold),
		pvController.FailedDeleteThreshold(opts.FailedDeleteThreshold),
		pvController.Threadiness(opts.Threadiness),
	)
	klog.Info("local pv provisioner started")
	pc.Run(ctx)
	klog.Info("local pv provisioner stopped")

	return nil
}
