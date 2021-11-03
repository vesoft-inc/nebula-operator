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

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

const (
	GraphdComponentType     = ComponentType("graphd")
	GraphdPortNameThrift    = "thrift"
	defaultGraphdPortThrift = 9669
	GraphdPortNameHTTP      = "http"
	defaultGraphdPortHTTP   = 19669
	GraphdPortNameHTTP2     = "http2"
	defaultGraphdPortHTTP2  = 19670
	defaultGraphdImage      = "vesoft/nebula-graphd"
)

var _ NebulaClusterComponentter = &graphdComponent{}

// +k8s:deepcopy-gen=false
func newGraphdComponent(nc *NebulaCluster) *graphdComponent {
	return &graphdComponent{
		baseComponent: baseComponent{
			nc:  nc,
			typ: GraphdComponentType,
		},
	}
}

type graphdComponent struct {
	baseComponent
}

func (c *graphdComponent) GetUpdateRevision() string {
	return c.nc.Status.Graphd.Workload.UpdateRevision
}

func (c *graphdComponent) GetReplicas() int32 {
	return *c.nc.Spec.Graphd.Replicas
}

func (c *graphdComponent) GetImage() string {
	return getImage(c.nc.Spec.Graphd.Image, c.nc.Spec.Graphd.Version, defaultGraphdImage)
}

func (c *graphdComponent) GetConfig() map[string]string {
	return c.nc.Spec.Graphd.Config
}

func (c *graphdComponent) GetConfigMapKey() string {
	return getConfigKey(c.Type().String())
}

func (c *graphdComponent) GetResources() *corev1.ResourceRequirements {
	return getResources(c.nc.Spec.Graphd.Resources)
}

func (c *graphdComponent) GetLogStorageClass() *string {
	scName := c.nc.Spec.Graphd.LogVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *graphdComponent) GetLogStorageResources() *corev1.ResourceRequirements {
	return c.nc.Spec.Graphd.LogVolumeClaim.Resources.DeepCopy()
}

func (c *graphdComponent) GetPodEnvVars() []corev1.EnvVar {
	return c.nc.Spec.Graphd.PodSpec.EnvVars
}

func (c *graphdComponent) GetPodAnnotations() map[string]string {
	return c.nc.Spec.Graphd.PodSpec.Annotations
}

func (c *graphdComponent) GetPodLabels() map[string]string {
	return c.nc.Spec.Graphd.PodSpec.Labels
}

func (c *graphdComponent) NodeSelector() map[string]string {
	selector := map[string]string{}
	for k, v := range c.nc.Spec.NodeSelector {
		selector[k] = v
	}
	for k, v := range c.nc.Spec.Graphd.PodSpec.NodeSelector {
		selector[k] = v
	}
	return selector
}

func (c *graphdComponent) Affinity() *corev1.Affinity {
	affinity := c.nc.Spec.Graphd.PodSpec.Affinity
	if affinity == nil {
		affinity = c.nc.Spec.Affinity
	}
	return affinity
}

func (c *graphdComponent) Tolerations() []corev1.Toleration {
	tolerations := c.nc.Spec.Graphd.PodSpec.Tolerations
	if len(tolerations) == 0 {
		return c.nc.Spec.Tolerations
	}
	return tolerations
}

func (c *graphdComponent) SidecarContainers() []corev1.Container {
	return c.nc.Spec.Graphd.PodSpec.SidecarContainers
}

func (c *graphdComponent) SidecarVolumes() []corev1.Volume {
	return c.nc.Spec.Graphd.PodSpec.SidecarVolumes
}

func (c *graphdComponent) ReadinessProbe() *corev1.Probe {
	return c.nc.Spec.Graphd.PodSpec.ReadinessProbe
}

func (c *graphdComponent) IsHeadlessService() bool {
	return false
}

func (c *graphdComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Graphd.Service == nil {
		return nil
	}
	return c.nc.Spec.Graphd.Service.ServiceSpec.DeepCopy()
}

func (c *graphdComponent) GetServiceName() string {
	return getServiceName(c.GetName(), c.IsHeadlessService())
}

func (c *graphdComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetServiceName(), c.GetNamespace())
}

func (c *graphdComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), c.IsHeadlessService())
}

func (c *graphdComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *graphdComponent) GetConnAddress(portName string) string {
	return getConnAddress(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *graphdComponent) GetPodConnAddresses(portName string, ordinal int32) string {
	return getPodConnAddress(c.GetPodFQDN(ordinal), c.GetPort(portName))
}

func (c *graphdComponent) GetHeadlessConnAddresses(portName string) []string {
	return getHeadlessConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.GetReplicas(),
		c.IsHeadlessService())
}

func (c *graphdComponent) IsReady() bool {
	return *c.nc.Spec.Graphd.Replicas == c.nc.Status.Graphd.Workload.ReadyReplicas
}

func (c *graphdComponent) GenerateLabels() map[string]string {
	return label.New().Cluster(c.GetClusterName()).Graphd()
}

func (c *graphdComponent) GenerateContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          GraphdPortNameThrift,
			ContainerPort: defaultGraphdPortThrift,
		},
		{
			Name:          GraphdPortNameHTTP,
			ContainerPort: defaultGraphdPortHTTP,
		},
		{
			Name:          GraphdPortNameHTTP2,
			ContainerPort: defaultGraphdPortHTTP2,
		},
	}
}

func (c *graphdComponent) GenerateVolumeMounts() []corev1.VolumeMount {
	componentType := c.Type().String()
	return []corev1.VolumeMount{
		{
			Name:      logVolume(componentType),
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		},
	}
}

func (c *graphdComponent) GenerateVolumes() []corev1.Volume {
	componentType := c.Type().String()
	return []corev1.Volume{
		{
			Name: logVolume(componentType),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: logVolume(componentType),
				},
			},
		},
	}
}

func (c *graphdComponent) GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error) {
	componentType := c.Type().String()
	logSC, logRes := c.GetLogStorageClass(), c.GetLogStorageResources()
	storageRequest, err := parseStorageRequest(logRes.Requests)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s, error: %v", componentType, err)
	}

	claims := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: logVolume(componentType),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        storageRequest,
				StorageClassName: logSC,
			},
		},
	}
	return claims, nil
}

func (c *graphdComponent) GenerateWorkload(
	gvk schema.GroupVersionKind,
	cm *corev1.ConfigMap,
	enableEvenPodsSpread bool) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm, enableEvenPodsSpread)
}

func (c *graphdComponent) GenerateService() *corev1.Service {
	return generateService(c)
}

func (c *graphdComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getConfigKey(c.Type().String())
	cm.Data[configKey] = GraphdConfigTemplate
	return cm
}

func (c *graphdComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Graphd = *status
}
