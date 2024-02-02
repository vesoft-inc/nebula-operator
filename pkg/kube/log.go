/*
Copyright 2024 Vesoft Inc.

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

package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	ErrInitializedNotReady = errors.New("pod initialized not ready")
	ErrContainerCreating   = errors.New("target container is creating")
)

func getPods(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (*corev1.PodList, error) {
	label := fmt.Sprintf("job-name=%s", jobName)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

func GetPodLog(ctx context.Context, client kubernetes.Interface, namespace, jobName, targetContainer string) (string, error) {
	pods, err := getPods(ctx, client, namespace, jobName)
	if err != nil || pods == nil || len(pods.Items) == 0 {
		klog.V(4).Infof("pods [%s/%s] are not found, %v", jobName, namespace, err)
		return "", nil
	}
	pod := pods.Items[0]

	if !isPodConditionInitializedTrue(pod.Status.Conditions) {
		return "", ErrInitializedNotReady
	}

	if getContainerWaitingReason(pod, targetContainer) == "ContainerCreating" {
		return "", ErrContainerCreating
	}

	req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: targetContainer})
	rc, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			return
		}
	}(rc)

	log, err := flushStream(rc)
	if err != nil {
		return "", err
	}

	return log, nil
}

func flushStream(rc io.ReadCloser) (string, error) {
	var buf = &bytes.Buffer{}
	_, err := io.Copy(buf, rc)
	if err != nil {
		return "", err
	}
	logContent := buf.String()
	return logContent, nil
}

func isPodConditionInitializedTrue(conditions []corev1.PodCondition) bool {
	podSchCond := getPodConditionFromList(conditions, corev1.PodInitialized)
	return podSchCond != nil && podSchCond.Status == corev1.ConditionTrue
}

func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) *corev1.PodCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func getContainerWaitingReason(pod corev1.Pod, containerName string) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.State.Waiting != nil {
				return cs.State.Waiting.Reason
			}
		}
	}
	return ""
}
