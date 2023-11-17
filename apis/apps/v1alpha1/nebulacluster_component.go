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
	"k8s.io/utils/pointer"
)

// +k8s:deepcopy-gen=false
type ComponentAccessor interface {
	Replicas() int32
	PodImage() string
	Resources() *corev1.ResourceRequirements
	PodLabels() map[string]string
	PodAnnotations() map[string]string
	PodEnvVars() []corev1.EnvVar
	NodeSelector() map[string]string
	Affinity() *corev1.Affinity
	Tolerations() []corev1.Toleration
	TopologySpreadConstraints(labels map[string]string) []corev1.TopologySpreadConstraint
	SecurityContext() *corev1.SecurityContext
	InitContainers() []corev1.Container
	SidecarContainers() []corev1.Container
	Volumes() []corev1.Volume
	VolumeMounts() []corev1.VolumeMount
	ReadinessProbe() *corev1.Probe
	LivenessProbe() *corev1.Probe
}

var _ ComponentAccessor = &componentAccessor{}

// +k8s:deepcopy-gen=false
type componentAccessor struct {
	nodeSelector              map[string]string
	affinity                  *corev1.Affinity
	tolerations               []corev1.Toleration
	topologySpreadConstraints []TopologySpreadConstraint
	componentSpec             *ComponentSpec
}

func (a *componentAccessor) Replicas() int32 {
	if a.componentSpec == nil {
		return 0
	}
	return pointer.Int32Deref(a.componentSpec.Replicas, 0)
}

func (a *componentAccessor) PodImage() string {
	if a.componentSpec == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s", a.componentSpec.Image, a.componentSpec.Version)
}

func (a *componentAccessor) Resources() *corev1.ResourceRequirements {
	if a.componentSpec == nil {
		return nil
	}
	return getResources(a.componentSpec.Resources)
}

func (a *componentAccessor) PodLabels() map[string]string {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.Labels
}

func (a *componentAccessor) PodAnnotations() map[string]string {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.Annotations
}

func (a *componentAccessor) PodEnvVars() []corev1.EnvVar {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.EnvVars
}

func (a *componentAccessor) NodeSelector() map[string]string {
	selector := map[string]string{}
	for k, v := range a.nodeSelector {
		selector[k] = v
	}
	if a.componentSpec != nil {
		for k, v := range a.componentSpec.NodeSelector {
			selector[k] = v
		}
	}
	return selector
}

func (a *componentAccessor) Affinity() *corev1.Affinity {
	if a.componentSpec == nil || a.componentSpec.Affinity == nil {
		return a.affinity
	}
	return a.componentSpec.Affinity
}

func (a *componentAccessor) Tolerations() []corev1.Toleration {
	if a.componentSpec == nil || len(a.componentSpec.Tolerations) == 0 {
		return a.tolerations
	}
	return a.componentSpec.Tolerations
}

func (a *componentAccessor) TopologySpreadConstraints(labels map[string]string) []corev1.TopologySpreadConstraint {
	tscs := a.topologySpreadConstraints
	if a.componentSpec != nil && len(a.componentSpec.TopologySpreadConstraints) > 0 {
		tscs = a.componentSpec.TopologySpreadConstraints
	}
	if len(tscs) == 0 {
		return nil
	}
	return getTopologySpreadConstraints(tscs, labels)
}

func (a *componentAccessor) SecurityContext() *corev1.SecurityContext {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.SecurityContext
}

func (a *componentAccessor) InitContainers() []corev1.Container {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.InitContainers
}

func (a *componentAccessor) SidecarContainers() []corev1.Container {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.SidecarContainers
}

func (a *componentAccessor) Volumes() []corev1.Volume {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.Volumes
}

func (a *componentAccessor) VolumeMounts() []corev1.VolumeMount {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.VolumeMounts
}

func (a *componentAccessor) ReadinessProbe() *corev1.Probe {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.ReadinessProbe
}

func (a *componentAccessor) LivenessProbe() *corev1.Probe {
	if a.componentSpec == nil {
		return nil
	}
	return a.componentSpec.LivenessProbe
}

// NebulaClusterComponent is the interface for component
// +k8s:deepcopy-gen=false
type NebulaClusterComponent interface {
	BaseComponent

	GetLogStorageResources() *corev1.ResourceRequirements
	GetDataStorageResources() (*corev1.ResourceRequirements, error)
	GetConfig() map[string]string
	GetConfigMapKey() string

	IsSSLEnabled() bool
	GetServiceSpec() *ServiceSpec
	GetHeadlessServiceName() string
	GetServiceFQDN() string
	GetPodFQDN(ordinal int32) string
	GetPort(portName string) int32
	GetConnAddress(portName string) string
	GetEndpoints(portName string) []string

	GenerateLabels() map[string]string
	GenerateContainerPorts() []corev1.ContainerPort
	GenerateVolumeMounts() []corev1.VolumeMount
	GenerateVolumes() []corev1.Volume
	GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error)
	GenerateWorkload(gvk schema.GroupVersionKind, cm *corev1.ConfigMap) (*unstructured.Unstructured, error)
	GenerateService() *corev1.Service
	GenerateHeadlessService() *corev1.Service
	GenerateConfigMap() *corev1.ConfigMap

	IsReady() bool
	GetUpdateRevision() string
	SetPhase(phase ComponentPhase)
	GetPhase() ComponentPhase
	IsSuspending() bool
	IsSuspended() bool
	IsAutoFailovering() bool
	SetWorkloadStatus(status *WorkloadStatus)
	UpdateComponentStatus(status *ComponentStatus)
}

// +k8s:deepcopy-gen=false
type BaseComponent interface {
	GraphdComponent() NebulaClusterComponent
	MetadComponent() NebulaClusterComponent
	StoragedComponent() NebulaClusterComponent
	ComponentSpec() ComponentAccessor
	ComponentType() ComponentType
	GetNebulaCluster() *NebulaCluster
	GetClusterName() string
	GetNamespace() string
	GetName() string
	GetPodName(ordinal int32) string
	GenerateOwnerReferences() []metav1.OwnerReference
}

var _ BaseComponent = &baseComponent{}

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

func (c *baseComponent) ComponentSpec() ComponentAccessor {
	var spec *ComponentSpec
	if c.typ == GraphdComponentType && c.nc.Spec.Graphd != nil {
		spec = &c.nc.Spec.Graphd.ComponentSpec
	}
	if c.typ == MetadComponentType && c.nc.Spec.Metad != nil {
		spec = &c.nc.Spec.Metad.ComponentSpec
	}
	if c.typ == StoragedComponentType && c.nc.Spec.Storaged != nil {
		spec = &c.nc.Spec.Storaged.ComponentSpec
	}
	return buildComponentAccessor(c.nc, spec)
}

func (c *baseComponent) GraphdComponent() NebulaClusterComponent {
	return newGraphdComponent(c.nc)
}

func (c *baseComponent) MetadComponent() NebulaClusterComponent {
	return newMetadComponent(c.nc)
}

func (c *baseComponent) StoragedComponent() NebulaClusterComponent {
	return newStoragedComponent(c.nc)
}

func (c *baseComponent) ComponentType() ComponentType {
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
	return getComponentName(c.GetClusterName(), c.ComponentType())
}

func (c *baseComponent) GetPodName(ordinal int32) string {
	return getPodName(c.GetName(), ordinal)
}

func (c *baseComponent) GenerateOwnerReferences() []metav1.OwnerReference {
	return c.nc.GenerateOwnerReferences()
}

func buildComponentAccessor(nc *NebulaCluster, componentSpec *ComponentSpec) ComponentAccessor {
	return &componentAccessor{
		nodeSelector:              nc.Spec.NodeSelector,
		affinity:                  nc.Spec.Affinity,
		tolerations:               nc.Spec.Tolerations,
		topologySpreadConstraints: nc.Spec.TopologySpreadConstraints,
		componentSpec:             componentSpec,
	}
}
