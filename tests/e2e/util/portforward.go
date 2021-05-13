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

package util

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwardOptions struct {
	Namespace    string
	PodName      string
	Config       *rest.Config
	Address      []string
	Ports        []string
	Out          io.Writer
	ErrOut       io.Writer
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func (o *PortForwardOptions) RunPortForward() error {
	clientSet, err := clientset.NewForConfig(o.Config)
	if err != nil {
		return err
	}
	pod, err := clientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	transport, upgrader, err := spdy.RoundTripperFor(o.Config)
	if err != nil {
		return err
	}
	req := clientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(o.Namespace).Name(pod.Name).SubResource("portforward")
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())
	fw, err := portforward.NewOnAddresses(dialer, o.Address, o.Ports, o.StopChannel, o.ReadyChannel, o.Out, o.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
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
