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

	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
)

const (
	GraphdComponentType  = ComponentType("graphd")
	GraphdPortNameThrift = "thrift"
	GraphdPortNameHTTP   = "http"
	GraphdPortNameHTTP2  = "http2"
	defaultGraphdImage   = "vesoft/nebula-graphd"
)

var _ NebulaClusterComponent = &graphdComponent{}

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

func (c *graphdComponent) GetConfig() map[string]string {
	return c.nc.Spec.Graphd.Config
}

func (c *graphdComponent) GetConfigMapKey() string {
	return getCmKey(c.ComponentType().String())
}

func (c *graphdComponent) GetLogStorageClass() *string {
	if c.nc.Spec.Graphd.LogVolumeClaim == nil {
		return nil
	}
	scName := c.nc.Spec.Graphd.LogVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *graphdComponent) GetLogStorageResources() *corev1.ResourceRequirements {
	if c.nc.Spec.Graphd.LogVolumeClaim == nil {
		return nil
	}
	return c.nc.Spec.Graphd.LogVolumeClaim.Resources.DeepCopy()
}

func (c *graphdComponent) GetDataStorageResources() (*corev1.ResourceRequirements, error) {
	return nil, nil
}

func (c *graphdComponent) IsSSLEnabled() bool {
	return (c.nc.Spec.Graphd.Config["enable_graph_ssl"] == "true" ||
		c.nc.Spec.Graphd.Config["enable_meta_ssl"] == "true" ||
		c.nc.Spec.Graphd.Config["enable_ssl"] == "true") &&
		c.nc.Spec.SSLCerts != nil
}

func (c *graphdComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Graphd.Service == nil {
		return nil
	}
	return c.nc.Spec.Graphd.Service.ServiceSpec.DeepCopy()
}

func (c *graphdComponent) GetHeadlessServiceName() string {
	return getServiceName(c.GetName(), true)
}

func (c *graphdComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetHeadlessServiceName(), c.GetNamespace())
}

func (c *graphdComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), true)
}

func (c *graphdComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *graphdComponent) GetConnAddress(portName string) string {
	return joinHostPort(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *graphdComponent) GetEndpoints(portName string) []string {
	return getConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.ComponentSpec().Replicas())
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
			ContainerPort: c.nc.Spec.Graphd.Port,
		},
		{
			Name:          GraphdPortNameHTTP,
			ContainerPort: c.nc.Spec.Graphd.HTTPPort,
		},
		{
			Name:          GraphdPortNameHTTP2,
			ContainerPort: c.nc.Spec.Graphd.HTTP2Port,
		},
	}
}

func (c *graphdComponent) GenerateVolumeMounts() []corev1.VolumeMount {
	componentType := c.ComponentType().String()
	mounts := make([]corev1.VolumeMount, 0)

	if c.nc.Spec.Graphd.LogVolumeClaim != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      logVolume(componentType),
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		})
	}

	if c.IsSSLEnabled() && c.nc.AutoMountServerCerts() {
		certMounts := []corev1.VolumeMount{
			{
				Name:      "server-crt",
				ReadOnly:  true,
				MountPath: "/usr/local/nebula/certs/server.crt",
				SubPath:   "server.crt",
			},
			{
				Name:      "server-key",
				ReadOnly:  true,
				MountPath: "/usr/local/nebula/certs/server.key",
				SubPath:   "server.key",
			},
			{
				Name:      "ca-crt",
				ReadOnly:  true,
				MountPath: "/usr/local/nebula/certs/ca.crt",
				SubPath:   "ca.crt",
			},
		}
		mounts = append(mounts, certMounts...)
	}

	return mounts
}

func (c *graphdComponent) GenerateVolumes() []corev1.Volume {
	componentType := c.ComponentType().String()
	volumes := make([]corev1.Volume, 0)

	if c.nc.Spec.Graphd.LogVolumeClaim != nil {
		volumes = append(volumes, corev1.Volume{
			Name: logVolume(componentType),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: logVolume(componentType),
				},
			},
		})
	}

	if c.IsSSLEnabled() && c.nc.AutoMountServerCerts() {
		certVolumes := []corev1.Volume{
			{
				Name: "server-crt",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.nc.Spec.SSLCerts.ServerSecret,
						Items: []corev1.KeyToPath{
							{
								Key:  c.nc.Spec.SSLCerts.ServerCert,
								Path: "server.crt",
							},
						},
					},
				},
			},
			{
				Name: "server-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.nc.Spec.SSLCerts.ServerSecret,
						Items: []corev1.KeyToPath{
							{
								Key:  c.nc.Spec.SSLCerts.ServerKey,
								Path: "server.key",
							},
						},
					},
				},
			},
			{
				Name: "ca-crt",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: c.nc.Spec.SSLCerts.CASecret,
						Items: []corev1.KeyToPath{
							{
								Key:  c.nc.Spec.SSLCerts.CACert,
								Path: "ca.crt",
							},
						},
					},
				},
			},
		}
		volumes = append(volumes, certVolumes...)
	}

	return volumes
}

func (c *graphdComponent) GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error) {
	if c.nc.Spec.Graphd.LogVolumeClaim == nil {
		return nil, nil
	}

	componentType := c.ComponentType().String()
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

func (c *graphdComponent) GenerateWorkload(gvk schema.GroupVersionKind, cm *corev1.ConfigMap) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm)
}

func (c *graphdComponent) GenerateService() *corev1.Service {
	return generateService(c, false)
}

func (c *graphdComponent) GenerateHeadlessService() *corev1.Service {
	return generateService(c, true)
}

func (c *graphdComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getCmKey(c.ComponentType().String())
	cm.Data[configKey] = GraphdConfigTemplate
	return cm
}

func (c *graphdComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Graphd = *status
}
