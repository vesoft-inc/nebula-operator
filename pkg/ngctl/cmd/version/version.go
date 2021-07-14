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

package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/kubernetes/pkg/apis/core"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util/ignore"
	operatorversion "github.com/vesoft-inc/nebula-operator/pkg/version"
)

var versionExample = templates.Examples(`# Print the cli and nebula operator version、ngctl version`)

type (
	// Options is a struct to support version command
	Options struct {
		ClientOnly bool

		kubeCli kubernetes.Interface
		genericclioptions.IOStreams
	}
)

// NewOptions returns initialized Options
func NewOptions(ioStreams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
	}
}

// NewCmdVersion returns a cobra command for fetching versions
func NewCmdVersion(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the cli and nebula operator version",
		Long:    "Print the cli and nebula operator version",
		Example: versionExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

// AddFlags add all the flags.
func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.ClientOnly, "client", o.ClientOnly, "If true, shows client version only (no server required).")
}

// Complete completes all the required options
func (o *Options) Complete(f cmdutil.Factory) error {
	var err error
	if o.ClientOnly {
		return nil
	}

	o.kubeCli, err = f.KubernetesClientSet()
	return err
}

// Run executes version command
func (o *Options) Run() error {
	clientVersion := operatorversion.Version()

	ignore.Fprintf(o.Out, "Client Version: %s\n", clientVersion)

	if o.ClientOnly {
		return nil
	}

	controllers, err := o.kubeCli.AppsV1().Deployments(core.NamespaceAll).List(context.TODO(), v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", label.NameLabelKey, "nebula-operator", label.ComponentLabelKey, "controller-manager"),
	})
	if err != nil {
		return err
	}

	if len(controllers.Items) == 0 {
		ignore.Fprintf(o.Out, "No Nebula Controller Manager found, please install first\n")
	} else if len(controllers.Items) == 1 {
		ignore.Fprintf(o.Out, "Nebula Controller Manager Version: %s\n", controllers.Items[0].Spec.Template.Spec.Containers[0].Image)
	} else {
		ignore.Fprintf(o.Out, "Nebula Controller Manager Versions:\n")
		for i := range controllers.Items {
			item := &controllers.Items[i]
			ignore.Fprintf(o.Out, "\t%s/%s: %s\n", item.Namespace, item.Name, item.Spec.Template.Spec.Containers[0].Image)
		}
	}

	return nil
}
