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

package predicates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

type topology struct {
	client client.Client
}

type Record struct {
	Index int
	Count int
}

func NewTopology(cli client.Client) Predicate {
	return &topology{client: cli}
}

func (t *topology) Filter(pod *corev1.Pod, preNodes []corev1.Node) ([]corev1.Node, error) {
	componentType := pod.Labels[label.ComponentLabelKey]
	clusterName := pod.Labels[label.ClusterLabelKey]

	nc := &v1alpha1.NebulaCluster{}
	if err := t.client.Get(context.TODO(), types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      clusterName,
	}, nc); err != nil {
		return nil, err
	}

	component, err := nc.ComponentByType(v1alpha1.ComponentType(componentType))
	if err != nil {
		klog.Errorf("get component %s failed: %v", componentType, err)
		return nil, err
	}

	podList := &corev1.PodList{}
	if err := t.client.List(context.TODO(), podList, client.InNamespace(pod.Namespace), client.MatchingLabels{
		label.ClusterLabelKey:   clusterName,
		label.ComponentLabelKey: componentType,
	}); err != nil {
		return nil, err
	}

	// ensures schedule by pod ordinal index
	nextPodName, err := getNextSchedulingPodName(component, podList.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to get the next scheduling pod for %s/%s, %v",
			pod.GetNamespace(), component.GetClusterName(), err)
	}
	if nextPodName != pod.GetName() {
		return nil, fmt.Errorf("waiting for Pod %s/%s scheduling", pod.GetNamespace(), nextPodName)
	}

	replicas := component.GetReplicas()
	maxSkew := int(math.Ceil(float64(replicas) / float64(len(preNodes))))

	nodeMap := make(map[string]*Record)
	for i := range preNodes {
		node := preNodes[i]
		nodeMap[node.Name] = &Record{
			Index: i,
			Count: 0,
		}
	}

	for i := range podList.Items {
		pod := podList.Items[i]
		if _, ok := nodeMap[pod.Spec.NodeName]; ok {
			nodeMap[pod.Spec.NodeName].Count++
		}
	}

	// find the node which has min pods and smaller than maxSkew
	min := math.MaxInt32
	for _, record := range nodeMap {
		if min > record.Count {
			min = record.Count
		}
	}

	resNodes := make([]corev1.Node, 0)
	resNames := make([]string, 0)
	for name, record := range nodeMap {
		if record.Count == min && record.Count < maxSkew {
			resNodes = append(resNodes, preNodes[record.Index])
			resNames = append(resNames, name)
		}
	}

	klog.Infof("candidate nodes %v", resNames)
	return resNodes, nil
}

func getNextSchedulingPodName(component v1alpha1.NebulaClusterComponentter, pods []corev1.Pod) (string, error) {
	podMap := make(map[string]*corev1.Pod, len(pods))
	for i := range pods {
		podMap[pods[i].GetName()] = &pods[i]
	}

	replicas := component.GetReplicas()
	for i := int32(0); i < replicas; i++ {
		podName := component.GetPodName(i)
		pod, ok := podMap[podName]
		if !ok {
			return podName, nil
		}
		// the first pod that not scheduling
		if pod.Spec.NodeName == "" {
			return podName, nil
		}
	}

	return "", errors.New("all pod scheduled")
}

// acquireLock ensures the scheduling is serial
// nolint: unused
func (t *topology) acquireLock(pod *corev1.Pod, pods []corev1.Pod) error {
	var currentPod, schedulingPod *corev1.Pod
	namespace := pod.GetNamespace()
	podName := pod.GetName()

	for i := range pods {
		if pods[i].GetName() == podName {
			currentPod = &pods[i]
		}
		if pods[i].Annotations[annotation.AnnPodSchedulingKey] != "" && schedulingPod == nil {
			schedulingPod = &pods[i]
		}
	}

	if currentPod == nil {
		klog.Infof("pod %s not found", podName)
		return fmt.Errorf("can't find current Pod %s/%s", namespace, podName)
	}

	// If there is no pod scheduling now, and then set the currentPod's annotation with AnnPodSchedulingKey
	if schedulingPod == nil {
		now := time.Now().Format(time.RFC3339)
		if currentPod.Annotations == nil {
			currentPod.Annotations = map[string]string{}
		}
		currentPod.Annotations[annotation.AnnPodSchedulingKey] = now
		err := t.client.Update(context.TODO(), currentPod)
		if err != nil {
			return err
		}
		klog.Infof("pod [%s/%s] set annotation successfully", namespace, podName)
		return nil
	}

	// If the currentPod is the schedulingPod, just return with no error for reentrant.
	if currentPod == schedulingPod {
		return nil
	}

	// If the scheduling pod already bind with node, and then remove the AnnPodSchedulingKey annotation
	if schedulingPod.Spec.NodeName != "" {
		delete(schedulingPod.Annotations, annotation.AnnPodSchedulingKey)
		err := t.client.Update(context.TODO(), schedulingPod)
		if err != nil {
			return err
		}
		klog.Infof("delete pod [%s/%s] annotation successfully", namespace, podName)
		return nil
	}

	return fmt.Errorf(
		"waiting for Pod %s/%s scheduling since %s",
		namespace, schedulingPod.GetName(), schedulingPod.Annotations[annotation.AnnPodSchedulingKey],
	)
}
