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

package options

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
)

var optionsExample = templates.Examples(`
		# Print flags inherited by all commands
		ngctl options`)

// NewCmdOptions implements the options command
func NewCmdOptions(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "options",
		Short:   "Print the list of flags inherited by all commands",
		Long:    "Print the list of flags inherited by all commands",
		Example: optionsExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cmd.Usage())
		},
	}

	cmd.SetOut(out)

	templates.UseOptionsTemplates(cmd)
	return cmd
}
