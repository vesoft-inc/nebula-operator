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
	MetadComponentType     = ComponentType("metad")
	MetadPortNameThrift    = "thrift"
	defaultMetadPortThrift = 9559
	MetadPortNameHTTP      = "http"
	defaultMetadPortHTTP   = 19559
	MetadPortNameHTTP2     = "http2"
	defaultMetadPortHTTP2  = 19560
	defaultMetadImage      = "vesoft/nebula-metad"
)

var _ NebulaClusterComponentter = &metadComponent{}

// +k8s:deepcopy-gen=false
func newMetadComponent(nc *NebulaCluster) *metadComponent {
	return &metadComponent{
		baseComponent: baseComponent{
			nc:  nc,
			typ: MetadComponentType,
		},
	}
}

type metadComponent struct {
	baseComponent
}

func (c *metadComponent) GetUpdateRevision() string {
	return c.nc.Status.Metad.Workload.UpdateRevision
}

func (c *metadComponent) GetReplicas() int32 {
	return *c.nc.Spec.Metad.Replicas
}

func (c *metadComponent) GetImage() string {
	return getImage(c.nc.Spec.Metad.Image, c.nc.Spec.Metad.Version, defaultMetadImage)
}

func (c *metadComponent) GetConfig() map[string]string {
	return c.nc.Spec.Metad.Config
}

func (c *metadComponent) GetConfigMapKey() string {
	return getConfigKey(c.Type().String())
}

func (c *metadComponent) GetResources() *corev1.ResourceRequirements {
	return getResources(c.nc.Spec.Metad.Resources)
}

func (c *metadComponent) GetLogStorageClass() *string {
	scName := c.nc.Spec.Metad.LogVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *metadComponent) GetDataStorageClass() *string {
	scName := c.nc.Spec.Metad.DataVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *metadComponent) GetLogStorageResources() *corev1.ResourceRequirements {
	return c.nc.Spec.Metad.LogVolumeClaim.Resources.DeepCopy()
}

func (c *metadComponent) GetDataStorageResources() *corev1.ResourceRequirements {
	return c.nc.Spec.Metad.DataVolumeClaim.Resources.DeepCopy()
}

func (c *metadComponent) GetPodEnvVars() []corev1.EnvVar {
	return c.nc.Spec.Metad.PodSpec.EnvVars
}

func (c *metadComponent) GetPodAnnotations() map[string]string {
	return c.nc.Spec.Metad.PodSpec.Annotations
}

func (c *metadComponent) GetPodLabels() map[string]string {
	return c.nc.Spec.Metad.PodSpec.Labels
}

func (c *metadComponent) NodeSelector() map[string]string {
	selector := map[string]string{}
	for k, v := range c.nc.Spec.NodeSelector {
		selector[k] = v
	}
	for k, v := range c.nc.Spec.Metad.PodSpec.NodeSelector {
		selector[k] = v
	}
	return selector
}

func (c *metadComponent) Affinity() *corev1.Affinity {
	affinity := c.nc.Spec.Metad.PodSpec.Affinity
	if affinity == nil {
		affinity = c.nc.Spec.Affinity
	}
	return affinity
}

func (c *metadComponent) Tolerations() []corev1.Toleration {
	tolerations := c.nc.Spec.Metad.PodSpec.Tolerations
	if len(tolerations) == 0 {
		return c.nc.Spec.Tolerations
	}
	return tolerations
}

func (c *metadComponent) SidecarContainers() []corev1.Container {
	return c.nc.Spec.Metad.PodSpec.SidecarContainers
}

func (c *metadComponent) SidecarVolumes() []corev1.Volume {
	return c.nc.Spec.Metad.PodSpec.SidecarVolumes
}

func (c *metadComponent) ReadinessProbe() *corev1.Probe {
	return c.nc.Spec.Metad.PodSpec.ReadinessProbe
}

func (c *metadComponent) IsHeadlessService() bool {
	return true
}

func (c *metadComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Metad.Service == nil {
		return nil
	}
	return c.nc.Spec.Metad.Service.DeepCopy()
}

func (c *metadComponent) GetServiceName() string {
	return getServiceName(c.GetName(), c.IsHeadlessService())
}

func (c *metadComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetServiceName(), c.GetNamespace())
}

func (c *metadComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), c.IsHeadlessService())
}

func (c *metadComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *metadComponent) GetConnAddress(portName string) string {
	return getConnAddress(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *metadComponent) GetPodConnAddresses(portName string, ordinal int32) string {
	return getPodConnAddress(c.GetPodFQDN(ordinal), c.GetPort(portName))
}

func (c *metadComponent) GetHeadlessConnAddresses(portName string) []string {
	return getHeadlessConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.GetReplicas(),
		c.IsHeadlessService())
}

func (c *metadComponent) IsReady() bool {
	return *c.nc.Spec.Metad.Replicas == c.nc.Status.Metad.Workload.ReadyReplicas
}

func (c *metadComponent) GenerateLabels() map[string]string {
	return label.New().Cluster(c.GetClusterName()).Metad()
}

func (c *metadComponent) GenerateContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          MetadPortNameThrift,
			ContainerPort: defaultMetadPortThrift,
		},
		{
			Name:          MetadPortNameHTTP,
			ContainerPort: defaultMetadPortHTTP,
		},
		{
			Name:          MetadPortNameHTTP2,
			ContainerPort: defaultMetadPortHTTP2,
		},
	}
}

func (c *metadComponent) GenerateVolumeMounts() []corev1.VolumeMount {
	componentType := c.Type().String()
	return []corev1.VolumeMount{
		{
			Name:      logVolume(componentType),
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		}, {
			Name:      dataVolume(componentType),
			MountPath: "/usr/local/nebula/data",
			SubPath:   "data",
		},
	}
}

func (c *metadComponent) GenerateVolumes() []corev1.Volume {
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
		{
			Name: dataVolume(componentType),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dataVolume(componentType),
				},
			},
		},
	}
}

// nolint: dupl
func (c *metadComponent) GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error) {
	componentType := c.Type().String()
	logSC, logRes := c.GetLogStorageClass(), c.GetLogStorageResources()
	logReq, err := parseStorageRequest(logRes.Requests)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s log volume, error: %v", componentType, err)
	}

	datSC, dataRes := c.GetDataStorageClass(), c.GetDataStorageResources()
	dataReq, err := parseStorageRequest(dataRes.Requests)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s data volume, error: %v", componentType, err)
	}

	claims := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: logVolume(componentType),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        logReq,
				StorageClassName: logSC,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: dataVolume(componentType),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        dataReq,
				StorageClassName: datSC,
			},
		},
	}
	return claims, nil
}

func (c *metadComponent) GenerateWorkload(
	gvk schema.GroupVersionKind,
	cm *corev1.ConfigMap,
	enableEvenPodsSpread bool) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm, enableEvenPodsSpread)
}

func (c *metadComponent) GenerateService() *corev1.Service {
	return generateService(c)
}

func (c *metadComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getConfigKey(c.Type().String())
	cm.Data[configKey] = MetadhConfigTemplate
	return cm
}

func (c *metadComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Metad = *status
}
