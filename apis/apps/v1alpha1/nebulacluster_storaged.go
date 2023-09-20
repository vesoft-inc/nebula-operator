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

var _ NebulaClusterComponent = &storagedComponent{}

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

func (c *storagedComponent) GetConfig() map[string]string {
	return c.nc.Spec.Storaged.Config
}

func (c *storagedComponent) GetConfigMapKey() string {
	return getCmKey(c.ComponentType().String())
}

func (c *storagedComponent) GetLogStorageClass() *string {
	if c.nc.Spec.Storaged.LogVolumeClaim == nil {
		return nil
	}
	scName := c.nc.Spec.Storaged.LogVolumeClaim.StorageClassName
	if scName == nil || *scName == "" {
		return nil
	}
	return scName
}

func (c *storagedComponent) GetDataStorageClass() *string {
	return nil
}

func (c *storagedComponent) GetLogStorageResources() *corev1.ResourceRequirements {
	if c.nc.Spec.Storaged.LogVolumeClaim == nil {
		return nil
	}
	return c.nc.Spec.Storaged.LogVolumeClaim.Resources.DeepCopy()
}

func (c *storagedComponent) GetDataStorageResources() (*corev1.ResourceRequirements, error) {
	resources := corev1.ResourceRequirements{}
	for i := range c.nc.Spec.Storaged.DataVolumeClaims {
		claim := c.nc.Spec.Storaged.DataVolumeClaims[i]
		storageRequest, err := parseStorageRequest(claim.Resources.Requests)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			resources = storageRequest
			continue
		}
		resources.Requests.Storage().Add(storageRequest.Requests.Storage().DeepCopy())
	}
	return &resources, nil
}

func (c *storagedComponent) IsSSLEnabled() bool {
	return (c.nc.Spec.Storaged.Config["enable_meta_ssl"] == "true" ||
		c.nc.Spec.Storaged.Config["enable_storage_ssl"] == "true" ||
		c.nc.Spec.Storaged.Config["enable_ssl"] == "true") &&
		c.nc.Spec.SSLCerts != nil
}

func (c *storagedComponent) GetServiceSpec() *ServiceSpec {
	if c.nc.Spec.Storaged.Service == nil {
		return nil
	}
	return c.nc.Spec.Storaged.Service.DeepCopy()
}

func (c *storagedComponent) GetHeadlessServiceName() string {
	return getServiceName(c.GetName(), true)
}

func (c *storagedComponent) GetServiceFQDN() string {
	return getServiceFQDN(c.GetHeadlessServiceName(), c.GetNamespace())
}

func (c *storagedComponent) GetPodFQDN(ordinal int32) string {
	return getPodFQDN(c.GetPodName(ordinal), c.GetServiceFQDN(), true)
}

func (c *storagedComponent) GetPort(portName string) int32 {
	return getPort(c.GenerateContainerPorts(), portName)
}

func (c *storagedComponent) GetConnAddress(portName string) string {
	return joinHostPort(c.GetServiceFQDN(), c.GetPort(portName))
}

func (c *storagedComponent) GetEndpoints(portName string) []string {
	return getConnAddresses(
		c.GetConnAddress(portName),
		c.GetName(),
		c.ComponentSpec().Replicas())
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
	componentType := c.ComponentType().String()
	mounts := make([]corev1.VolumeMount, 0)

	for i := range c.nc.Spec.Storaged.DataVolumeClaims {
		volumeName := storageDataVolume(componentType, i)
		mountPath := "/usr/local/nebula/data"
		if i > 0 {
			mountPath = fmt.Sprintf("/usr/local/nebula/data%d", i)
		}
		subPath := "data"
		if i > 0 {
			subPath = fmt.Sprintf("data%d", i)
		}
		mount := corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
			SubPath:   subPath,
		}
		mounts = append(mounts, mount)
	}

	if c.nc.Spec.Storaged.LogVolumeClaim != nil {
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

func (c *storagedComponent) GenerateVolumes() []corev1.Volume {
	componentType := c.ComponentType().String()
	volumes := make([]corev1.Volume, 0)

	for i := range c.nc.Spec.Storaged.DataVolumeClaims {
		volumeName := storageDataVolume(componentType, i)
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: volumeName,
				},
			},
		}
		volumes = append(volumes, volume)
	}

	if c.nc.Spec.Storaged.LogVolumeClaim != nil {
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

func (c *storagedComponent) GenerateVolumeClaim() ([]corev1.PersistentVolumeClaim, error) {
	componentType := c.ComponentType().String()
	claims := make([]corev1.PersistentVolumeClaim, 0)

	dataClaims, err := storageDataVolumeClaims(c.nc.Spec.Storaged.DataVolumeClaims, componentType)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s data volumes, error: %v", componentType, err)
	}
	claims = append(claims, dataClaims...)

	if c.nc.Spec.Storaged.LogVolumeClaim != nil {
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

func (c *storagedComponent) GenerateWorkload(gvk schema.GroupVersionKind, cm *corev1.ConfigMap) (*unstructured.Unstructured, error) {
	return generateWorkload(c, gvk, cm)
}

func (c *storagedComponent) GenerateService() *corev1.Service {
	return nil
}

func (c *storagedComponent) GenerateHeadlessService() *corev1.Service {
	return generateService(c, true)
}

func (c *storagedComponent) GenerateConfigMap() *corev1.ConfigMap {
	cm := generateConfigMap(c)
	configKey := getCmKey(c.ComponentType().String())
	cm.Data[configKey] = StoragedConfigTemplate
	return cm
}

func (c *storagedComponent) UpdateComponentStatus(status *ComponentStatus) {
	c.nc.Status.Storaged.ComponentStatus = *status
}

func storageDataVolumeClaims(storageClaims []StorageClaim, componentType string) ([]corev1.PersistentVolumeClaim, error) {
	var pvcs []corev1.PersistentVolumeClaim
	for i := range storageClaims {
		storageRequest, err := parseStorageRequest(storageClaims[i].Resources.Requests)
		if err != nil {
			return nil, err
		}
		volumeName := storageDataVolume(componentType, i)
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: volumeName},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				StorageClassName: storageClaims[i].StorageClassName,
				Resources:        storageRequest,
			},
		})
	}
	return pvcs, nil
}
