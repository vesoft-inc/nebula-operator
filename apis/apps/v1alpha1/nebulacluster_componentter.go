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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NebulaClusterComponentter is the interface for component
// +k8s:deepcopy-gen=false
type NebulaClusterComponentter interface {
	baseComponentter

	GetUpdateRevision() string
	GetReplicas() int32
	GetImage() string
	GetConfig() map[string]string
	GetConfigMapKey() string
	GetResources() *corev1.ResourceRequirements
	GetPodEnvVars() []corev1.EnvVar
	GetPodAnnotations() map[string]string
	GetPodLabels() map[string]string
	GetLogStorageResources() *corev1.ResourceRequirements
	GetDataStorageResources() (*corev1.ResourceRequirements, error)
	NodeSelector() map[string]string
	Affinity() *corev1.Affinity
	Tolerations() []corev1.Toleration
	InitContainers() []corev1.Container
	SidecarContainers() []corev1.Container
	SidecarVolumes() []corev1.Volume
	ReadinessProbe() *corev1.Probe
	IsHeadlessService() bool
	GetServiceSpec() *ServiceSpec
	GetServiceName() string
	GetServiceFQDN() string
	GetPodFQDN(int32) string
	GetPort(string) int32
	GetConnAddress(string) string
	GetEndpoints(string) []string

	IsReady() bool

	GenerateLabels() map[string]string
	GenerateContainerPorts() []corev1.ContainerPort
	GenerateVolumeMounts() []corev1.VolumeMount
	GenerateVolumes() []corev1.Volume
	GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error)
	GenerateWorkload(gvk schema.GroupVersionKind, cm *corev1.ConfigMap, enableEvenPodsSpread bool) (*unstructured.Unstructured, error)
	GenerateService() *corev1.Service
	GenerateConfigMap() *corev1.ConfigMap

	UpdateComponentStatus(status *ComponentStatus)
}

// +k8s:deepcopy-gen=false
type baseComponentter interface {
	GraphdComponent() NebulaClusterComponentter
	MetadComponent() NebulaClusterComponentter
	StoragedComponent() NebulaClusterComponentter
	Type() ComponentType
	GetNebulaCluster() *NebulaCluster
	GetClusterName() string
	GetNamespace() string
	GetName() string
	GetPodName(int32) string
	GenerateOwnerReferences() []metav1.OwnerReference
}

var _ baseComponentter = &baseComponent{}

// ComponentType is the type of NebulaCluster Component: graphd, metad or storaged
// +k8s:deepcopy-gen=false
type ComponentType string

func (typ ComponentType) String() string {
	return string(typ)
}

// +k8s:deepcopy-gen=false
type baseComponent struct {
	nc  *NebulaCluster
	typ ComponentType
}

func (c *baseComponent) GraphdComponent() NebulaClusterComponentter {
	return newGraphdComponent(c.nc)
}

func (c *baseComponent) MetadComponent() NebulaClusterComponentter {
	return newMetadComponent(c.nc)
}

func (c *baseComponent) StoragedComponent() NebulaClusterComponentter {
	return newStoragedComponent(c.nc)
}

func (c *baseComponent) Type() ComponentType {
	return c.typ
}

func (c *baseComponent) GetNebulaCluster() *NebulaCluster {
	return c.nc
}

func (c *baseComponent) GetClusterName() string {
	return c.nc.GetClusterName()
}

func (c *baseComponent) GetNamespace() string {
	return c.nc.Namespace
}

func (c *baseComponent) GetName() string {
	return getComponentName(c.GetClusterName(), c.Type())
}

func (c *baseComponent) GetPodName(ordinal int32) string {
	return getPodName(c.GetName(), ordinal)
}

func (c *baseComponent) GenerateOwnerReferences() []metav1.OwnerReference {
	return c.nc.GenerateOwnerReferences()
}
