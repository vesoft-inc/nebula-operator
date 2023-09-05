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

package component

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

const defaultConsoleImage = "vesoft/nebula-console"

type nebulaConsole struct {
	clientSet kube.ClientSet
}

func NewNebulaConsole(clientSet kube.ClientSet) ReconcileManager {
	return &nebulaConsole{clientSet: clientSet}
}

func (c *nebulaConsole) Reconcile(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Console == nil {
		return nil
	}
	return c.syncConsolePod(nc)
}

func (c *nebulaConsole) syncConsolePod(nc *v1alpha1.NebulaCluster) error {
	newPod := c.generatePod(nc)
	oldPod, err := c.clientSet.Pod().GetPod(newPod.Namespace, newPod.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	notExist := apierrors.IsNotFound(err)
	if notExist {
		if err := setPodLastAppliedConfigAnnotation(newPod); err != nil {
			return err
		}
		return c.clientSet.Pod().CreatePod(newPod)
	}
	return updatePod(c.clientSet, newPod, oldPod)
}

func (c *nebulaConsole) generatePod(nc *v1alpha1.NebulaCluster) *corev1.Pod {
	mounts := make([]corev1.VolumeMount, 0)
	if nc.IsGraphdSSLEnabled() || nc.IsClusterSSLEnabled() {
		certMounts := []corev1.VolumeMount{
			{
				Name:      "client-crt",
				ReadOnly:  true,
				MountPath: "/tmp/client.crt",
				SubPath:   "client.crt",
			},
			{
				Name:      "client-key",
				ReadOnly:  true,
				MountPath: "/tmp/client.key",
				SubPath:   "client.key",
			},
			{
				Name:      "client-ca-crt",
				ReadOnly:  true,
				MountPath: "/tmp/ca.crt",
				SubPath:   "ca.crt",
			},
		}
		mounts = append(mounts, certMounts...)
	}

	cmd := []string{
		"nebula-console",
		"-addr",
		nc.GraphdComponent().GetServiceName(),
		"-port",
		strconv.Itoa(int(nc.GraphdComponent().GetPort(v1alpha1.GraphdPortNameThrift))),
	}

	username := "root"
	if nc.Spec.Console.Username != "" {
		username = nc.Spec.Console.Username
	}
	password := "nebula"
	if nc.Spec.Console.Password != "" {
		password = nc.Spec.Console.Password
	}
	cmd = append(cmd, "-u", username, "-p", password)

	if nc.IsGraphdSSLEnabled() || nc.IsClusterSSLEnabled() {
		cmd = append(cmd, "-enable_ssl", "-ssl_cert_path", "/tmp/client.crt", "-ssl_private_key_path",
			"/tmp/client.key", "-ssl_root_ca_path", "/tmp/ca.crt")
	}
	if nc.InsecureSkipVerify() {
		cmd = append(cmd, "-ssl_insecure_skip_verify")
	}

	container := corev1.Container{
		Name:            "console",
		Image:           getConsoleImage(nc.Spec.Console),
		ImagePullPolicy: corev1.PullAlways,
		Command:         cmd,
		VolumeMounts:    mounts,
		Stdin:           true,
		StdinOnce:       true,
		TTY:             true,
	}

	volumes := make([]corev1.Volume, 0)
	if nc.IsGraphdSSLEnabled() || nc.IsClusterSSLEnabled() {
		certVolumes := v1alpha1.GetClientCertsVolume(nc.Spec.SSLCerts)
		volumes = append(volumes, certVolumes...)
	}

	podName := nc.GetName() + "-console"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            podName,
			Namespace:       nc.GetNamespace(),
			Labels:          c.getConsoleLabels(nc),
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		Spec: corev1.PodSpec{
			SchedulerName:      nc.Spec.SchedulerName,
			NodeSelector:       c.getNodeSelector(nc),
			Containers:         []corev1.Container{container},
			ImagePullSecrets:   nc.Spec.ImagePullSecrets,
			ServiceAccountName: v1alpha1.NebulaServiceAccountName,
			Volumes:            volumes,
		},
	}
}

func (c *nebulaConsole) getConsoleLabels(nc *v1alpha1.NebulaCluster) map[string]string {
	selector := label.New().Cluster(nc.GetName()).Console()
	labels := selector.Copy().Labels()
	return labels
}

func (c *nebulaConsole) getNodeSelector(nc *v1alpha1.NebulaCluster) map[string]string {
	selector := map[string]string{}
	for k, v := range nc.Spec.NodeSelector {
		selector[k] = v
	}
	consoleSelector := nc.Spec.Console.NodeSelector
	if consoleSelector != nil {
		for k, v := range consoleSelector {
			selector[k] = v
		}
	}
	return selector
}

func getConsoleImage(console *v1alpha1.ConsoleSpec) string {
	image := defaultConsoleImage
	if console.Image != "" {
		image = console.Image
	}
	if console.Version != "" {
		image = fmt.Sprintf("%s:%s", image, console.Version)
	}
	return image
}

type FakeNebulaConsole struct {
	err error
}

func NewFakeNebulaConsole() *FakeNebulaConsole {
	return &FakeNebulaConsole{}
}

func (f *FakeNebulaConsole) SetReconcileError(err error) {
	f.err = err
}

func (f *FakeNebulaConsole) Reconcile(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
