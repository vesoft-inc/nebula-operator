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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

const (
	StoragedComponentType     = ComponentType("storaged")
	StoragedPortNameThrift    = "thrift"
	defaultStoragedPortThrift = 9779
	StoragedPortNameHTTP      = "http"
	defaultStoragedPortHTTP   = 19779
	StoragedPortNameHTTP2     = "http2"
	defaultStoragedPortHTTP2  = 19780
	StoragedPortNameAdmin     = "admin"
	defaultStoragedPortAdmin  = 9778
	defaultStoragedImage      = "vesoft/nebula-storaged"
)

var _ NebulaClusterComponentter = &storagedComponent{}

// +k8s:deepcopy-gen=false
func newStoragedComponent(nc *NebulaCluster) *storagedComponent {
	return &storagedComponent{
		baseComponent: baseComponent{
			nc:  nc,
			typ: StoragedComponentType,
		},
	}
}

type storagedComponent struct {
	baseComponent
}

func (c *storagedComponent) GetUpdateRevision() string {
	return c.nc.Status.Storaged.Workload.UpdateRevision
}

func (c *storagedComponent) GetReplicas() int32 {
	return *c.nc.Spec.Storaged.Replicas
}

func (c *storagedComponent) GetImage() string {
	return getImage(c.nc.Spec.Storaged.Image, c.nc.Spec.Storaged.Version, defaultStoragedImage)
}

func (c *storagedComponent) GetConfig() map[string]string {
	return c.nc.Spec.Storaged.Config
}

func (c *storagedComponent) GetConfigMapKey() string {
	return getConfigKey(c.Type().String())
}

func (c *storagedComponent) GetResources() *corev1.ResourceRequirements {
	return getResources(c.nc.Spec.Storaged.Resources)
}

func (c *storagedComponent) GetStorageClass() *string {
	scName := c.nc.Spec.Storaged.StorageClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *storagedComponent) GetStorageResources() *corev1.ResourceRequirements {
	return c.nc.Spec.Storaged.StorageClaim.Resources.DeepCopy()
}

func (c *storagedComponent) GetPodEnvVars() []corev1.EnvVar {
	return c.nc.Spec.Storaged.PodSpec.EnvVars
}

func (c *storagedComponent) GetPodAnnotations() map[string]string {
	return c.nc.Spec.Graphd.PodSpec.Annotations
}

func (c *storagedComponent) GetPodLabels() map[string]string {
	return c.nc.Spec.Graphd.PodSpec.Labels
}

func (c *storagedComponent) IsHeadlessService() bool {
	return true
}

func (c *storagedComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Storaged.Service != nil {
		return nil
	}
	return c.nc.Spec.Storaged.Service.DeepCopy()
}

func (c *storagedComponent) GetServiceName() string {
	return getServiceName(c.GetName(), c.IsHeadlessService())
}

func (c *storagedComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetServiceName(), c.GetNamespace())
}

func (c *storagedComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), c.IsHeadlessService())
}

func (c *storagedComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *storagedComponent) GetConnAddress(portName string) string {
	return getConnAddress(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *storagedComponent) GetPodConnAddresses(portName string, ordinal int32) string {
	return getPodConnAddress(c.GetPodFQDN(ordinal), c.GetPort(portName))
}

func (c *storagedComponent) GetHeadlessConnAddresses(portName string) []string {
	return getHeadlessConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.GetReplicas(),
		c.IsHeadlessService())
}

func (c *storagedComponent) IsReady() bool {
	return *c.nc.Spec.Storaged.Replicas == c.nc.Status.Storaged.Workload.ReadyReplicas
}

func (c *storagedComponent) GenerateLabels() map[string]string {
	return label.New().Cluster(c.GetClusterName()).Storaged()
}

func (c *storagedComponent) GenerateContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          StoragedPortNameThrift,
			ContainerPort: defaultStoragedPortThrift,
		},
		{
			Name:          StoragedPortNameHTTP,
			ContainerPort: defaultStoragedPortHTTP,
		},
		{
			Name:          StoragedPortNameHTTP2,
			ContainerPort: defaultStoragedPortHTTP2,
		},
		{
			Name:          StoragedPortNameAdmin,
			ContainerPort: defaultStoragedPortAdmin,
		},
	}
}

func (c *storagedComponent) GenerateVolumeMounts() []corev1.VolumeMount {
	componentType := c.Type().String()
	return []corev1.VolumeMount{
		{
			Name:      componentType,
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		}, {
			Name:      componentType,
			MountPath: "/usr/local/nebula/data",
			SubPath:   "data",
		},
	}
}

func (c *storagedComponent) GenerateVolumes() []corev1.Volume {
	componentType := c.Type().String()
	return []corev1.Volume{
		{
			Name: componentType,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: componentType,
				},
			},
		},
	}
}

func (c *storagedComponent) GenerateWorkload(
	gvk schema.GroupVersionKind,
	cm *corev1.ConfigMap,
	enableEvenPodsSpread bool) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm, enableEvenPodsSpread)
}

func (c *storagedComponent) GenerateService() *corev1.Service {
	return generateService(c)
}

func (c *storagedComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getConfigKey(c.Type().String())
	cm.Data[configKey] = StoragedConfigTemplate
	return cm
}

func (c *storagedComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Storaged = *status
}
