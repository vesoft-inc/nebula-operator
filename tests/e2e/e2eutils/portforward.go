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

package e2eutils

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type (
	PortForwardOptions struct {
		RestConfig *rest.Config
		Namespace  string
		Name       string
		Address    string
		Ports      []int
		Out        io.Writer
		ErrOut     io.Writer
	}

	PortForwardOption func(*PortForwardOptions)
)

func WithRestConfig(restConfig *rest.Config) PortForwardOption {
	return func(o *PortForwardOptions) {
		o.RestConfig = restConfig
	}
}

func WithPod(namespace, name string) PortForwardOption {
	return func(o *PortForwardOptions) {
		o.Namespace = namespace
		o.Name = name
	}
}

func WithAddress(address string) PortForwardOption {
	return func(o *PortForwardOptions) {
		o.Address = address
	}
}

func WithPorts(ports ...int) PortForwardOption {
	return func(o *PortForwardOptions) {
		o.Ports = append(o.Ports, ports...)
	}
}

func (o *PortForwardOptions) WithOptions(opts ...PortForwardOption) *PortForwardOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func PortForward(opts ...PortForwardOption) (localPorts []int, stopChan chan struct{}, err error) {
	o := (&PortForwardOptions{
		Address: "localhost",
		Out:     os.Stdout,
		ErrOut:  os.Stderr,
	}).WithOptions(opts...)

	return o.runPortForward()
}

func (o *PortForwardOptions) runPortForward() (localPorts []int, stopChan chan struct{}, err error) {
	stopChan = make(chan struct{}, 1)
	localPorts = make([]int, len(o.Ports))
	for i := range o.Ports {
		localPorts[i], err = getFreePort(o.Address)
		if err != nil {
			return nil, nil, err
		}
	}

	readyCh := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		if err := o.portForward(localPorts, stopChan, readyCh); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		close(stopChan)
		return nil, nil, err
	case <-readyCh:
		return localPorts, stopChan, nil
	}
}

func (o *PortForwardOptions) portForward(localPorts []int, stopChan, readyChan chan struct{}) error {
	clientSet, err := clientset.NewForConfig(o.RestConfig)
	if err != nil {
		return err
	}
	pod, err := clientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	transport, upgrader, err := spdy.RoundTripperFor(o.RestConfig)
	if err != nil {
		return err
	}

	strPorts := make([]string, len(o.Ports))
	for i := range o.Ports {
		strPorts[i] = fmt.Sprintf("%d:%d", localPorts[i], o.Ports[i])
	}
	req := clientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(o.Namespace).Name(pod.Name).SubResource("portforward")
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())
	fw, err := portforward.NewOnAddresses(dialer, []string{o.Address}, strPorts, stopChan, readyChan, o.Out, o.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func getFreePort(address string) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", address))
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = l.Close()
	}()
	return l.Addr().(*net.TCPAddr).Port, nil
}
