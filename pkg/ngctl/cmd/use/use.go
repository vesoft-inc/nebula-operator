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

package use

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/config"
)

var (
	useLong = templates.LongDesc(`
		Specify a nebula cluster to use.
		
		By using a certain cluster, you may omit --nebulacluster option
		in many control commands.`)

	useExample = templates.Examples(`
		# specify a nebula cluster to use
		ngctl use demo-cluster

		# specify kubernetes context and namespace
		ngctl use --namespace=demo-ns demo-cluster`)

	useUsage = "expected 'use CLUSTER_NAME' for the use command"
)

type (
	// Options is a struct to support use command
	Options struct {
		Namespace               string
		NebulaClusterName       string
		NebulaClusterConfigFile string

		runtimeCli client.Client
		genericclioptions.IOStreams
	}
)

// NewOptions returns initialized Options
func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: streams,
	}
}

// NewCmdUse returns a cobra command for specify a nebula cluster to use
func NewCmdUse(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "use",
		Short:   "specify a nebula cluster to use",
		Long:    useLong,
		Example: useExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Validate(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	f.AddFlags(cmd)

	return cmd
}

// Complete completes all the required options
func (o *Options) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.NebulaClusterName, o.Namespace, err = f.GetNebulaClusterNameAndNamespace(false, args)
	if err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	o.runtimeCli, err = f.ToRuntimeClient()
	if err != nil {
		return err
	}

	o.NebulaClusterConfigFile, err = f.GetNebulaClusterConfigFile()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate(cmd *cobra.Command) error {
	if o.NebulaClusterName == "" {
		return cmdutil.UsageErrorf(cmd, useUsage)
	}
	return nil
}

// Run executes use command
func (o *Options) Run() error {
	var nc appsv1alpha1.NebulaCluster
	key := client.ObjectKey{Namespace: o.Namespace, Name: o.NebulaClusterName}
	if err := o.runtimeCli.Get(context.TODO(), key, &nc); err != nil {
		return err
	}

	if err := config.NewNebulaClusterConfig(nc.Namespace, nc.Name).SaveToFile(o.NebulaClusterConfigFile); err != nil {
		return err
	}
	_, err := fmt.Fprintf(o.Out, "Nebula cluster config switch to %s/%s, save to %q.\n",
		nc.Namespace, nc.Name, o.NebulaClusterConfigFile)
	return err
}
