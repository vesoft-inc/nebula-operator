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

package executor

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	cmdattach "k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/logs"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/interrupt"

	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util/ignore"
)

type (
	PodExecutor struct {
		Pod              *corev1.Pod
		ContainerName    string
		KubeCli          kubernetes.Interface
		RestClientGetter genericclioptions.RESTClientGetter
		RestConfig       *rest.Config

		genericclioptions.IOStreams
	}
)

func NewPodExecutor(
	pod *corev1.Pod, containerName string,
	restClientGetter genericclioptions.RESTClientGetter,
	ioStreams genericclioptions.IOStreams,
) *PodExecutor {
	return &PodExecutor{
		Pod:              pod,
		ContainerName:    containerName,
		RestClientGetter: restClientGetter,
		IOStreams:        ioStreams,
	}
}

func (e *PodExecutor) Execute(ctx context.Context) error {
	var err error
	e.RestConfig, err = e.RestClientGetter.ToRESTConfig()
	if err != nil {
		return err
	}
	e.KubeCli, err = kubernetes.NewForConfig(e.RestConfig)
	if err != nil {
		return err
	}

	pod, err := e.KubeCli.CoreV1().Pods(e.Pod.Namespace).Create(ctx, e.Pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	e.Pod = pod

	if e.ContainerName == "" {
		container, err := e.getContainer()
		if err != nil {
			return err
		}
		e.ContainerName = container.Name
	}

	defer func() {
		_ = e.destroy(ctx)
	}()
	return e.attachPod(ctx)
}

func (e *PodExecutor) destroy(ctx context.Context) error {
	return e.removePod(ctx)
}

func (e *PodExecutor) removePod(ctx context.Context) error {
	return e.KubeCli.CoreV1().Pods(e.Pod.Namespace).Delete(ctx, e.Pod.Name, metav1.DeleteOptions{})
}

func (e *PodExecutor) attachPod(ctx context.Context) error {
	pod, err := e.waitForContainer(ctx)
	if err != nil {
		return err
	}

	e.Pod = pod
	attachOpts := cmdattach.NewAttachOptions(e.IOStreams)

	attachOpts.Pod = pod
	attachOpts.PodName = pod.Name
	attachOpts.Namespace = pod.Namespace
	attachOpts.ContainerName = e.ContainerName
	attachOpts.Config = e.RestConfig
	attachOpts.StreamOptions.Stdin = true
	attachOpts.StreamOptions.TTY = true
	attachOpts.StreamOptions.Quiet = false

	status := e.getContainerStatus()
	if status == nil {
		return fmt.Errorf("error getting container status of container name %q: %+v", e.ContainerName, err)
	}
	if status.State.Terminated != nil {
		ignore.Fprintf(e.ErrOut, "Container terminated, falling back to logs")
		return e.logBack()
	}

	if err := attachOpts.Run(); err != nil {
		ignore.Fprintf(e.ErrOut, "Error attaching, falling back to logs: %v\n", err)
		return e.logBack()
	}
	return nil
}

func (e *PodExecutor) waitForContainer(ctx context.Context) (*corev1.Pod, error) {
	ctx, cancel := watchtools.ContextWithOptionalTimeout(ctx, 0*time.Second)
	defer cancel()

	pod := e.Pod
	podClient := e.KubeCli.CoreV1()

	fieldSelector := fields.OneTermEqualSelector("metadata.name", pod.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return podClient.Pods(pod.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return podClient.Pods(pod.Namespace).Watch(ctx, options)
		},
	}

	intr := interrupt.New(nil, cancel)
	var result *corev1.Pod
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(ev watch.Event) (bool, error) {
			if ev.Type == watch.Deleted {
				return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
			}
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				return false, fmt.Errorf("watch did not return a pod: %v", ev.Object)
			}
			e.Pod = pod

			s := e.getContainerStatus()
			if s == nil {
				return false, nil
			}
			if s.State.Running != nil || s.State.Terminated != nil {
				return true, nil
			}
			return false, nil
		})
		if ev != nil {
			result = ev.Object.(*corev1.Pod)
		}
		return err
	})

	return result, err
}

func (e *PodExecutor) getContainer() (*corev1.Container, error) {
	pod := e.Pod
	containerName := e.ContainerName
	if containerName != "" {
		for i := range pod.Spec.Containers {
			if pod.Spec.Containers[i].Name == containerName {
				return &pod.Spec.Containers[i], nil
			}
		}
		for i := range pod.Spec.InitContainers {
			if pod.Spec.InitContainers[i].Name == containerName {
				return &pod.Spec.InitContainers[i], nil
			}
		}
		for i := range pod.Spec.EphemeralContainers {
			if pod.Spec.EphemeralContainers[i].Name == containerName {
				return (*corev1.Container)(&pod.Spec.EphemeralContainers[i].EphemeralContainerCommon), nil
			}
		}
		return nil, fmt.Errorf("container not found (%s)", e.ContainerName)
	}

	return &pod.Spec.Containers[0], nil
}

func (e *PodExecutor) getContainerStatus() *corev1.ContainerStatus {
	pod := e.Pod
	allContainerStatus := [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses,
		pod.Status.ContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	}
	for _, statusSlice := range allContainerStatus {
		for i := range statusSlice {
			if statusSlice[i].Name == e.ContainerName {
				return &statusSlice[i]
			}
		}
	}
	return nil
}

func (e *PodExecutor) logBack() error {
	pod := e.Pod
	container, err := e.getContainer()
	if err != nil {
		return err
	}

	requests, err := polymorphichelpers.LogsForObjectFn(e.RestClientGetter, pod,
		&corev1.PodLogOptions{Container: container.Name}, 5*time.Second, false)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if err := logs.DefaultConsumeRequest(request, e.Out); err != nil {
			return err
		}
	}

	return nil
}
