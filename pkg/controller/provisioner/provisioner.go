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

package provisioner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-helpers/storage/volume"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/v9/controller"
)

type ActionType string

const (
	ActionTypeCreate = "create"
	ActionTypeDelete = "delete"
)

const (
	helperScriptDir        = "/tmp"
	helperDiskVolName      = "disk"
	helperScriptVolName    = "script"
	cmdTimeoutSeconds      = 30
	helperPodNameMaxLength = 128
	nodeNameAnnotationKey  = "local.pv.provisioner/selected-node"
)

type LocalVolumeProvisioner struct {
	*CacheConfig
	ctx        context.Context
	kubeClient *clientset.Clientset

	namespace          string
	helperImage        string
	imagePullSecret    string
	configMapName      string
	serviceAccountName string
}

func NewProvisioner(ctx context.Context, kubeClient *clientset.Clientset, namespace, helperImage, configMapName,
	serviceAccountName string) *LocalVolumeProvisioner {
	return &LocalVolumeProvisioner{
		ctx:                ctx,
		kubeClient:         kubeClient,
		namespace:          namespace,
		helperImage:        helperImage,
		configMapName:      configMapName,
		serviceAccountName: serviceAccountName,
	}
}

func (p *LocalVolumeProvisioner) SetImagePullSecret(secret string) {
	p.imagePullSecret = secret
}

func (p *LocalVolumeProvisioner) Provision(ctx context.Context, opts pvController.ProvisionOptions) (*corev1.PersistentVolume, pvController.ProvisioningState, error) {
	pvc := opts.PVC
	node := opts.SelectedNode
	storageClass := opts.StorageClass
	if storageClass.ReclaimPolicy != nil {
		policy := *storageClass.ReclaimPolicy
		if policy != corev1.PersistentVolumeReclaimRetain && policy != corev1.PersistentVolumeReclaimDelete {
			return nil, pvController.ProvisioningFinished, fmt.Errorf("ReclaimPolicy only supportes Retain and Delete")
		}
	}
	if pvc.Spec.Selector != nil {
		return nil, pvController.ProvisioningFinished, fmt.Errorf("claim.Spec.Selector is not supported")
	}
	for _, accessMode := range pvc.Spec.AccessModes {
		if accessMode != corev1.ReadWriteOnce && accessMode != corev1.ReadWriteOncePod {
			return nil, pvController.ProvisioningFinished, fmt.Errorf("NodePath only supports ReadWriteOnce and ReadWriteOncePod (1.22+) access modes")
		}
	}
	if node == nil {
		return nil, pvController.ProvisioningFinished, fmt.Errorf("configuration error, no node was specified")
	}

	pvs := p.Cache.ListPVs()
	for _, pv := range pvs {
		if err := volume.CheckNodeAffinity(pv, node.Labels); err == nil {
			return nil, pvController.ProvisioningReschedule, fmt.Errorf("PV %s node affinity match node %s, should be rescheduled to other nodes", pv.Name, node.Name)
		}
	}

	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	provisionCmd := []string{"/bin/bash", fmt.Sprintf("%s/setup", helperScriptDir)}
	path := DefaultMountPoint
	if storageClass.Parameters != nil {
		if _, ok := storageClass.Parameters["mountPoint"]; ok {
			path = storageClass.Parameters["mountPoint"]
		}
	}

	name := opts.PVName
	if err := p.createHelperPod(ActionTypeCreate, provisionCmd, VolumeOptions{
		Name:        name,
		Path:        path,
		Mode:        *pvc.Spec.VolumeMode,
		SizeInBytes: storage.Value(),
		Node:        node.Name,
	}); err != nil {
		return nil, pvController.ProvisioningFinished, err
	}

	fs := corev1.PersistentVolumeFilesystem
	nodeAffinity, err := generateVolumeNodeAffinity(node)
	if err != nil {
		return nil, pvController.ProvisioningFinished, err
	}

	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{nodeNameAnnotationKey: node.Name},
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *opts.StorageClass.ReclaimPolicy,
			AccessModes:                   pvc.Spec.AccessModes,
			VolumeMode:                    &fs,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: pvc.Spec.Resources.Requests[corev1.ResourceStorage],
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path:   path,
					FSType: pointer.String(DefaultFsType),
				},
			},
			NodeAffinity: nodeAffinity,
		},
	}, pvController.ProvisioningFinished, nil
}

func (p *LocalVolumeProvisioner) Delete(ctx context.Context, pv *corev1.PersistentVolume) error {
	path, node, err := p.getPathAndNodeForPV(pv)
	if err != nil {
		klog.Error(err)
		return err
	}
	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		klog.Infof("deleting volume %s at %s:%s", pv.Name, node, path)
		storage := pv.Spec.Capacity[corev1.ResourceStorage]
		cleanupCmd := []string{"/bin/bash", fmt.Sprintf("%s/teardown", helperScriptDir)}
		if err := p.createHelperPod(ActionTypeDelete, cleanupCmd, VolumeOptions{
			Name:        pv.Name,
			Path:        path,
			Mode:        *pv.Spec.VolumeMode,
			SizeInBytes: storage.Value(),
			Node:        node,
		}); err != nil {
			klog.Errorf("clean up volume %s failed: %v", pv.Name, err)
			return err
		}
		return nil
	}
	klog.Infof("retained volume %s", pv.Name)
	return nil
}

func (p *LocalVolumeProvisioner) getPathAndNodeForPV(pv *corev1.PersistentVolume) (path, node string, err error) {
	if pv.Spec.PersistentVolumeSource.Local == nil {
		return "", "", fmt.Errorf("volume %s VolumeSource not set", pv.Name)
	}
	path = pv.Spec.PersistentVolumeSource.Local.Path

	nodeAffinity := pv.Spec.NodeAffinity
	if nodeAffinity == nil {
		return "", "", fmt.Errorf("volume %s NodeAffinity not set", pv.Name)
	}
	required := nodeAffinity.Required
	if required == nil {
		return "", "", fmt.Errorf("volume %s NodeAffinity.Required not set", pv.Name)
	}

	node = ""
	for _, selectorTerm := range required.NodeSelectorTerms {
		for _, expression := range selectorTerm.MatchExpressions {
			if expression.Key == corev1.LabelHostname && expression.Operator == corev1.NodeSelectorOpIn {
				if len(expression.Values) != 1 {
					return "", "", fmt.Errorf("volume %s multiple values for the node affinity", pv.Name)
				}
				node = expression.Values[0]
				break
			}
		}
		if node != "" {
			break
		}
	}
	if node == "" {
		return "", "", fmt.Errorf("volume %s cannot find affinited node", pv.Name)
	}
	return path, node, nil
}

func (p *LocalVolumeProvisioner) createHelperPod(action ActionType, cmd []string, opts VolumeOptions) error {
	if opts.Name == "" || opts.Path == "" || opts.Node == "" {
		return fmt.Errorf("invalid empty name or path or node")
	}
	if !filepath.IsAbs(opts.Path) {
		return fmt.Errorf("volume path %s is not absolute", opts.Path)
	}

	opts.Path = filepath.Clean(opts.Path)
	parentDir, volumeDir := filepath.Split(opts.Path)
	parentDir = strings.TrimSuffix(parentDir, string(filepath.Separator))
	volumeDir = strings.TrimSuffix(volumeDir, string(filepath.Separator))
	if parentDir == "" || volumeDir == "" {
		return fmt.Errorf("path %s is invalid, parentDir or volumeDir is empty", opts.Path)
	}

	podName := getHelperPodName(action, opts.Name)
	if len(podName) > helperPodNameMaxLength {
		podName = podName[:helperPodNameMaxLength]
	}
	mountPropagation := corev1.MountPropagationHostToContainer
	helperPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: p.namespace,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: helperDiskVolName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: parentDir,
						},
					},
				},
				{
					Name: helperScriptVolName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: p.configMapName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  "setup",
									Path: "setup",
								},
								{
									Key:  "teardown",
									Path: "teardown",
								},
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "helper-pod",
					Image:   p.helperImage,
					Command: cmd,
					Env: []corev1.EnvVar{
						{
							Name:  LocalFilesystemEnv,
							Value: opts.Path,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.Bool(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:             helperDiskVolName,
							MountPath:        parentDir,
							MountPropagation: &mountPropagation,
						},
						{
							Name:      helperScriptVolName,
							MountPath: helperScriptDir,
						},
					},
				},
			},
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: p.serviceAccountName,
			Tolerations: []corev1.Toleration{
				{
					Key:      corev1.TaintNodeDiskPressure,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeName: opts.Node,
		},
	}

	if p.imagePullSecret != "" {
		helperPod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: p.imagePullSecret,
			},
		}
	}

	klog.Infof("create the helper pod %s into %s", helperPod.Name, p.namespace)
	pod, err := p.kubeClient.CoreV1().Pods(p.namespace).Create(context.TODO(), helperPod, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	defer func() {
		if err := saveHelperPodLogs(p.kubeClient, pod); err != nil {
			klog.Error(err.Error())
		}
		if err := p.kubeClient.CoreV1().Pods(p.namespace).Delete(context.TODO(), helperPod.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("unable to delete the helper pod: %v", err)
		}
	}()

	completed := false
	for i := 0; i < cmdTimeoutSeconds; i++ {
		if pod, err := p.kubeClient.CoreV1().Pods(p.namespace).Get(context.TODO(), helperPod.Name, metav1.GetOptions{}); err != nil {
			return err
		} else if pod.Status.Phase == corev1.PodSucceeded {
			completed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !completed {
		return fmt.Errorf("create process timeout after %d seconds", cmdTimeoutSeconds)
	}

	klog.Infof("volume %s has been %v on %s:%s", opts.Name, action, opts.Node, opts.Path)
	return nil
}

func saveHelperPodLogs(c clientset.Interface, pod *corev1.Pod) error {
	podLogOpts := corev1.PodLogOptions{
		Container: "helper-pod",
	}
	request := c.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	readCloser, err := request.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("faild to open stream: %v", err)
	}
	defer readCloser.Close()

	logs, err := flushStream(readCloser)
	if err != nil {
		return err
	}

	klog.Infof("Start of %s logs", pod.Name)
	if len(logs) > 0 {
		podLogs := strings.Split(strings.Trim(logs, "\n"), "\n")
		for _, log := range podLogs {
			klog.Info(log)
		}
	}
	klog.Infof("End of %s logs", pod.Name)
	return nil
}

func flushStream(rc io.ReadCloser) (string, error) {
	var buf = &bytes.Buffer{}
	_, err := io.Copy(buf, rc)
	if err != nil {
		return "", fmt.Errorf("failed to copy buffer: %v", err)
	}
	logContent := buf.String()
	return logContent, nil
}

func getHelperPodName(action ActionType, pvName string) string {
	return "helper-pod" + "-" + string(action) + "-" + pvName
}

func generateVolumeNodeAffinity(node *corev1.Node) (*corev1.VolumeNodeAffinity, error) {
	if node.Labels == nil {
		return nil, fmt.Errorf("node %s does not have labels", node.Name)
	}
	nodeValue, found := node.Labels[corev1.LabelHostname]
	if !found {
		return nil, fmt.Errorf("node %s does not have expected label %s", node.Name, corev1.LabelHostname)
	}
	return &corev1.VolumeNodeAffinity{
		Required: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelHostname,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{nodeValue},
						},
					},
				},
			},
		},
	}, nil
}
