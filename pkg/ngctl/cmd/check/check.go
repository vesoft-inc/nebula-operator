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

package check

import (
	"bytes"
	"context"
	"text/tabwriter"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util/ignore"
)

var (
	checkLong = templates.LongDesc(`
		Check whether the specified nebula cluster resources are ready.`)

	checkExample = templates.Examples(`
		# check whether the specified nebula cluster is ready
		ngctl check 
		
		# check specified nebula cluster pods
		ngctl check pods --nebulacluster=nebula`)
)

type CheckOptions struct {
	Namespace         string
	NebulaClusterName string
	ResourceType      string
	AllNamespaces     bool

	runtimeCli client.Client
	genericclioptions.IOStreams
}

func NewCheckOptions(streams genericclioptions.IOStreams) *CheckOptions {
	return &CheckOptions{
		IOStreams: streams,
	}
}

// NewCmdCheck returns a cobra command for check whether nebula cluster resources are ready
func NewCmdCheck(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewCheckOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "check",
		Short:   "check whether nebula cluster resources are ready",
		Long:    checkLong,
		Example: checkExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Validate(cmd))
			cmdutil.CheckErr(o.RunCheck())
		},
	}

	f.AddFlags(cmd)
	return cmd
}

// Complete completes all the required options
func (o *CheckOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error

	if o.NebulaClusterName, err = f.GetNebulaClusterName(); err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	if o.Namespace, err = f.GetNamespace(); err != nil {
		return err
	}

	if len(args) > 0 {
		o.ResourceType = args[0]
	}

	o.runtimeCli, err = f.ToRuntimeClient()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the provided options
func (o *CheckOptions) Validate(cmd *cobra.Command) error {
	if o.NebulaClusterName == "" {
		return cmdutil.UsageErrorf(cmd, "expected specify nebula cluster like 'ngctl check resource --nebulacluster=CLUSTER_NAME' for the check command, or using 'ngctl use' \nto set nebula cluster first.")
	}

	if o.ResourceType == "" {
		o.ResourceType = "nebulacluster"
	}

	return nil
}

// Run executes check command
func (o *CheckOptions) RunCheck() error {
	switch o.ResourceType {
	case "nebulaclusters", "nebulacluster", "nc":
		{
			str, err := o.CheckNebulaCluster()
			if err != nil {
				return err
			}
			ignore.Fprintf(o.Out, "%s\n", str)
		}
	case "pod", "pods":
		{
			str, err := o.CheckPods()
			if err != nil {
				return err
			}
			ignore.Fprintf(o.Out, "%s\n", str)

		}
	}

	return nil
}

func (o *CheckOptions) CheckNebulaCluster() (string, error) {
	var nc appsv1alpha1.NebulaCluster
	key := client.ObjectKey{Namespace: o.Namespace, Name: o.NebulaClusterName}
	if err := o.runtimeCli.Get(context.TODO(), key, &nc); err != nil {
		return "", err
	}
	for _, cond := range nc.Status.Conditions {
		if cond.Type == v1alpha1.NebulaClusterReady {
			return cond.Message, nil
		}
	}
	return "", nil
}

func (o *CheckOptions) CheckPods() (string, error) {
	selector, err := label.New().Cluster(o.NebulaClusterName).Selector()
	if err != nil {
		return "", err
	}

	var pods corev1.PodList
	listOptions := client.ListOptions{
		LabelSelector: selector,
		Namespace:     o.Namespace,
	}
	if err := o.runtimeCli.List(context.TODO(), &pods, &listOptions); err != nil {
		return "", err
	}

	allWork := true
	tw := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	tw.Init(buf, 0, 8, 4, ' ', 0)

	ignore.Fprintf(tw, "Trouble Pods:\n")
	ignore.Fprintf(tw, "\tPodName\tPhase\tConditionType\tMessage\n")
	ignore.Fprintf(tw, "\t-------\t------\t-------------\t-------\n")

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			allWork = false
			for _, cond := range pod.Status.Conditions {
				if cond.Status != corev1.ConditionTrue {
					ignore.Fprintf(tw, "\t%s", pod.Name)
					ignore.Fprintf(tw, "\t%s\t%s\t%s\n", pod.Status.Phase, cond.Type, cond.Message)
				}
			}
		}
	}

	_ = tw.Flush()

	if allWork {
		return "All pods are running", nil
	} else {
		return buf.String(), nil
	}
}
