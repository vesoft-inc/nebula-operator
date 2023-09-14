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

package console

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	cmdutil "github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/executor"
)

const (
	consoleLong = `
		Open console to the specified nebula cluster.
`
	consoleExample = `
		# Open console to the current nebula cluster, which is set by 'ngctl use' command
		ngctl console

		# Open console to the specified nebula cluster
		ngctl console CLUSTER_NAME
`
	consoleUsage = `expected 'console CLUSTER_NAME' for the console command, or using 'ngctl use'
to set nebula cluster first.
`
	consolePodGenerateNameFmt = "console-%s-"
	consoleContainerName      = "console"
	consoleDefaultImage       = "vesoft/nebula-console:nightly"
)

type (
	// Options is a struct to support console command
	Options struct {
		Namespace         string
		NebulaClusterName string
		Image             string
		ImagePullPolicy   corev1.PullPolicy

		User     string
		Password string

		runtimeCli       client.Client
		restClientGetter genericclioptions.RESTClientGetter
		genericclioptions.IOStreams
	}
)

// NewOptions returns initialized Options
func NewOptions(ioStreams genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams: ioStreams,
		Image:     consoleDefaultImage,
		User:      "root",
		Password:  "*",
	}
}

// NewCmdConsole returns a cobra command for open console to the specified nebula cluster
func NewCmdConsole(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "console",
		Short:   "Open console to the nebula cluster",
		Long:    consoleLong,
		Example: consoleExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	f.AddFlags(cmd)
	o.AddFlags(cmd)

	return cmd
}

// AddFlags add all the flags.
func (o *Options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Image, "image", o.Image, "Container image to use for open console to the nebula cluster.")
	cmd.Flags().String("image-pull-policy", "",
		"The image pull policy for the container."+
			" If left empty, this value will not be specified by the client and defaulted by the server.")
	cmd.Flags().StringVarP(&o.User, "user", "u", o.User, "The NebulaGraph login user name.")
	cmd.Flags().StringVarP(&o.Password, "password", "p", o.Password, "The NebulaGraph login password.")
}

// Complete completes all the required options
func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.NebulaClusterName, o.Namespace, err = f.GetNebulaClusterNameAndNamespace(true, args)
	if err != nil && !cmdutil.IsErNotSpecified(err) {
		return err
	}

	var imagePullPolicy string
	imagePullPolicy, err = cmd.Flags().GetString("image-pull-policy")
	if err != nil {
		return err
	}

	o.ImagePullPolicy = corev1.PullPolicy(imagePullPolicy)

	o.runtimeCli, err = f.ToRuntimeClient()
	if err != nil {
		return err
	}

	o.restClientGetter = f

	return nil
}

// Validate validates the provided options
func (o *Options) Validate(cmd *cobra.Command) error {
	if o.NebulaClusterName == "" {
		return cmdutil.UsageErrorf(cmd, consoleUsage)
	}

	switch o.ImagePullPolicy {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever, "":
	default:
		return fmt.Errorf("invalid image pull policy: %s", o.ImagePullPolicy)
	}

	if o.User == "" {
		return fmt.Errorf("the NebulaGraph login user name cannot be empty")
	}

	if o.Password == "" {
		return fmt.Errorf("the NebulaGraph login password cannot be empty")
	}

	return nil
}

// Run executes console command
func (o *Options) Run() error {
	pod, err := o.generateConsolePod()
	if err != nil {
		return err
	}

	e := executor.NewPodExecutor(pod, consoleContainerName, o.restClientGetter, o.IOStreams)
	return e.Execute(context.TODO())
}

func (o *Options) generateConsolePod() (*corev1.Pod, error) {
	var nc appsv1alpha1.NebulaCluster
	key := client.ObjectKey{Namespace: o.Namespace, Name: o.NebulaClusterName}
	if err := o.runtimeCli.Get(context.TODO(), key, &nc); err != nil {
		return nil, err
	}

	consoleContainer := corev1.Container{
		Name:  consoleContainerName,
		Image: o.Image,
		Command: []string{
			"nebula-console",
			"-u", o.User,
			"-p", o.Password,
			"--addr", nc.GetGraphdServiceName(),
			"--port", strconv.Itoa(int(nc.GraphdComponent().GetPort(appsv1alpha1.GraphdPortNameThrift))),
		},
		Stdin:     true,
		StdinOnce: true,
		TTY:       true,
	}

	if o.ImagePullPolicy != "" {
		consoleContainer.ImagePullPolicy = o.ImagePullPolicy
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf(consolePodGenerateNameFmt, o.NebulaClusterName),
			Namespace:    o.Namespace,
			Labels: map[string]string{
				label.ClusterLabelKey:          nc.GetClusterName(),
				label.ComponentLabelKey:        "console",
				"app.kubernetes.io/managed-by": "ngctl",
			},
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{consoleContainer},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	return pod, nil
}
