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
	MetadComponentType     = ComponentType("metad")
	MetadPortNameThrift    = "thrift"
	defaultMetadPortThrift = 9559
	MetadPortNameHTTP      = "http"
	defaultMetadPortHTTP   = 19559
	defaultMetadImage      = "vesoft/nebula-metad"
)

var _ NebulaClusterComponent = &metadComponent{}

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
	if c.nc.Status.Metad.Workload == nil {
		return ""
	}
	return c.nc.Status.Metad.Workload.UpdateRevision
}

func (c *metadComponent) GetConfig() map[string]string {
	return c.nc.Spec.Metad.Config
}

func (c *metadComponent) GetConfigMapKey() string {
	return getCmKey(c.ComponentType().String())
}

func (c *metadComponent) GetResources() *corev1.ResourceRequirements {
	return getResources(c.nc.Spec.Metad.Resources)
}

func (c *metadComponent) GetLogStorageClass() *string {
	if c.nc.Spec.Metad.LogVolumeClaim == nil {
		return nil
	}
	scName := c.nc.Spec.Metad.LogVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *metadComponent) GetDataStorageClass() *string {
	if c.nc.Spec.Metad.DataVolumeClaim == nil {
		return nil
	}
	scName := c.nc.Spec.Metad.DataVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *metadComponent) GetLogStorageResources() *corev1.ResourceRequirements {
	if c.nc.Spec.Metad.LogVolumeClaim == nil {
		return nil
	}
	return c.nc.Spec.Metad.LogVolumeClaim.Resources.DeepCopy()
}

func (c *metadComponent) GetDataStorageResources() (*corev1.ResourceRequirements, error) {
	if c.nc.Spec.Metad.DataVolumeClaim == nil {
		return nil, nil
	}
	return c.nc.Spec.Metad.DataVolumeClaim.Resources.DeepCopy(), nil
}

func (c *metadComponent) IsSSLEnabled() bool {
	return (c.nc.Spec.Metad.Config["enable_meta_ssl"] == "true" ||
		c.nc.Spec.Metad.Config["enable_storage_ssl"] == "true" ||
		c.nc.Spec.Metad.Config["enable_ssl"] == "true") &&
		c.nc.Spec.SSLCerts != nil
}

func (c *metadComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Metad.Service == nil {
		return nil
	}
	return c.nc.Spec.Metad.Service.DeepCopy()
}

func (c *metadComponent) GetHeadlessServiceName() string {
	return getServiceName(c.GetName(), true)
}

func (c *metadComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetHeadlessServiceName(), c.GetNamespace())
}

func (c *metadComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), true)
}

func (c *metadComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *metadComponent) GetConnAddress(portName string) string {
	return joinHostPort(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *metadComponent) GetEndpoints(portName string) []string {
	return getConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.ComponentSpec().Replicas())
}

func (c *metadComponent) IsReady() bool {
	if c.nc.Status.Metad.Workload == nil {
		return false
	}
	return *c.nc.Spec.Metad.Replicas == c.nc.Status.Metad.Workload.ReadyReplicas &&
		rollingUpdateDone(c.nc.Status.Metad.Workload)
}

func (c *metadComponent) GenerateLabels() map[string]string {
	return label.New().Cluster(c.GetClusterName()).Metad()
}

func (c *metadComponent) GenerateContainerPorts() []corev1.ContainerPort {
	thriftPort, err := parseCustomPort(defaultMetadPortThrift, c.GetConfig()["port"])
	if err != nil {
		return nil
	}

	httpPort, err := parseCustomPort(defaultMetadPortHTTP, c.GetConfig()["ws_http_port"])
	if err != nil {
		return nil
	}

	return []corev1.ContainerPort{
		{
			Name:          MetadPortNameThrift,
			ContainerPort: thriftPort,
		},
		{
			Name:          MetadPortNameHTTP,
			ContainerPort: httpPort,
		},
	}
}

func (c *metadComponent) GenerateVolumeMounts() []corev1.VolumeMount {
	componentType := c.ComponentType().String()
	mounts := []corev1.VolumeMount{
		{
			Name:      dataVolume(componentType),
			MountPath: "/usr/local/nebula/data",
			SubPath:   "data",
		},
	}

	if c.nc.Spec.Metad.LogVolumeClaim != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      logVolume(componentType),
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		})
	}

	if c.nc.Spec.Metad.License != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "nebula-license",
			ReadOnly:  true,
			MountPath: "/usr/local/nebula/nebula.license",
			SubPath:   "nebula.license",
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

func (c *metadComponent) GenerateVolumes() []corev1.Volume {
	componentType := c.ComponentType().String()
	volumes := []corev1.Volume{
		{
			Name: dataVolume(componentType),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dataVolume(componentType),
				},
			},
		},
	}

	if c.nc.Spec.Metad.LogVolumeClaim != nil {
		volumes = append(volumes, corev1.Volume{
			Name: logVolume(componentType),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: logVolume(componentType),
				},
			},
		})
	}

	if c.nc.Spec.Metad.License != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "nebula-license",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: c.nc.Spec.Metad.License.SecretName,
					Items: []corev1.KeyToPath{
						{
							Key:  c.nc.Spec.Metad.License.LicenseKey,
							Path: c.nc.Spec.Metad.License.LicenseKey,
						},
					},
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

func (c *metadComponent) GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error) {
	componentType := c.ComponentType().String()
	claims := make([]corev1.PersistentVolumeClaim, 0)

	dataRes, err := c.GetDataStorageResources()
	if err != nil {
		return nil, err
	}
	dataSC := c.GetDataStorageClass()
	dataReq, err := parseStorageRequest(dataRes.Requests)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s data volume, error: %v", componentType, err)
	}

	claims = append(claims, corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolume(componentType),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        dataReq,
			StorageClassName: dataSC,
		},
	})

	if c.nc.Spec.Metad.LogVolumeClaim != nil {
		logSC, logRes := c.GetLogStorageClass(), c.GetLogStorageResources()
		logReq, err := parseStorageRequest(logRes.Requests)
		if err != nil {
			return nil, fmt.Errorf("cannot parse storage request for %s log volume, error: %v", componentType, err)
		}

		claims = append(claims, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: logVolume(componentType),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        logReq,
				StorageClassName: logSC,
			},
		})
	}

	return claims, nil
}

func (c *metadComponent) GenerateWorkload(gvk schema.GroupVersionKind, cm *corev1.ConfigMap) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm)
}

func (c *metadComponent) GenerateService() *corev1.Service {
	return nil
}

func (c *metadComponent) GenerateHeadlessService() *corev1.Service {
	return generateService(c, true)
}

func (c *metadComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getCmKey(c.ComponentType().String())
	cm.Data[configKey] = MetadhConfigTemplate
	return cm
}

func (c *metadComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Metad = *status
}

func (c *metadComponent) SetWorkloadStatus(status *WorkloadStatus) {
	c.nc.Status.Metad.Workload = status
}

func (c *metadComponent) GetPhase() ComponentPhase {
	return c.nc.Status.Metad.Phase
}

func (c *metadComponent) SetPhase(phase ComponentPhase) {
	c.nc.Status.Metad.Phase = phase
}

func (c *metadComponent) IsSuspending() bool {
	return c.nc.Status.Metad.Phase == SuspendPhase
}

func (c *metadComponent) IsSuspended() bool {
	if !c.IsSuspending() {
		return false
	}
	if c.nc.IsSuspendEnabled() && c.nc.Status.Metad.Workload != nil {
		return false
	}
	return true
}
