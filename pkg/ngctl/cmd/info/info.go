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

package info

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
)

const (
	infoLong = `
		Get specified nebula cluster information.
`
	infoExample = `
		# get current nebula cluster information, which is set by 'ngctl use' command
		ngctl info
		
		# get specified nebula cluster information
		ngctl info CLUSTER_NAME
`
	infoUsage = `expected 'info CLUSTER_NAME' for the info command, or using 'ngctl use' 
to set nebula cluster first.
`
)

type (
	// Options is a struct to support info command
	Options struct {
		Namespace         string
		NebulaClusterName string

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

// NewCmdInfo returns a cobra command for get specified nebula cluster information
func NewCmdInfo(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "info",
		Short:   "Show nebula cluster information",
		Long:    infoLong,
		Example: infoExample,
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
	o.NebulaClusterName, o.Namespace, err = f.GetNebulaClusterNameAndNamespace(true, args)
	if err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	o.runtimeCli, err = f.ToRuntimeClient()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the provided options
func (o *Options) Validate(cmd *cobra.Command) error {
	if o.NebulaClusterName == "" {
		return cmdutil.UsageErrorf(cmd, infoUsage)
	}
	return nil
}

// Run executes info command
func (o *Options) Run() error {
	nci, err := NewNebulaClusterInfo(o.NebulaClusterName, o.Namespace, o.runtimeCli)
	if err != nil {
		return err
	}
	str, err := nci.Render()
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(o.Out, str)
	return err
}
