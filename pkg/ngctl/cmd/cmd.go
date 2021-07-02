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

package cmd

import (
	"flag"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/console"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/info"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/list"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/options"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/use"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/version"
)

const (
	nkcLongDescription = `
		"ngctl"(NebulaGraph Kubernetes Control) controls the Nebula cluster manager on Kubernetes.
`
)

func NewNgctlCmd(in io.Reader, out, err io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   "ngctl",
		Short: "NebulaGraph kubernetes control.",
		Long:  nkcLongDescription,
		Run:   runHelp,
	}

	flags := cmds.PersistentFlags()

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(cmds.PersistentFlags())

	cmds.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: err}

	groups := templates.CommandGroups{
		{
			Message: "Cluster Management Commands:",
			Commands: []*cobra.Command{
				console.NewCmdConsole(f, ioStreams),
				info.NewCmdInfo(f, ioStreams),
				list.NewCmdList(f, ioStreams),
				use.NewCmdUse(f, ioStreams),
				version.NewCmdVersion(f, ioStreams),
			},
		},
	}

	groups.Add(cmds)

	filters := []string{"options"}

	templates.ActsAsRootCommand(cmds, filters, groups...)

	cmds.AddCommand(options.NewCmdOptions(ioStreams.Out))

	return cmds
}

func runHelp(cmd *cobra.Command, _ []string) {
	cmdutil.CheckErr(cmd.Help())
}
