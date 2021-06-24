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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	kruisepub "github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

func getComponentName(clusterName string, typ ComponentType) string {
	return fmt.Sprintf("%s-%s", clusterName, typ)
}

func getPodName(componentName string, ordinal int32) string {
	return fmt.Sprintf("%s-%d", componentName, ordinal)
}

func getImage(image, version, defaultImage string) string {
	if image == "" {
		image = defaultImage
	}
	if version == "" {
		return image
	}
	return fmt.Sprintf("%s:%s", image, version)
}

func getServiceName(componentName string, isHeadless bool) string {
	if isHeadless {
		return fmt.Sprintf("%s-headless", componentName)
	}
	return fmt.Sprintf("%s-svc", componentName)
}

func getServiceFQDN(service, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.%s", service, namespace, getKubernetesClusterDomain())
}

func getPodFQDN(podName, serviceFQDN string, isHeadless bool) string {
	if isHeadless {
		return fmt.Sprintf("%s.%s", podName, serviceFQDN)
	}
	return ""
}

func getPort(ports []corev1.ContainerPort, portName string) int32 {
	for _, port := range ports {
		if port.Name == portName {
			return port.ContainerPort
		}
	}
	return 0
}

func getConnAddress(serviceFQDN string, port int32) string {
	return joinHostPort(serviceFQDN, port)
}

func getPodConnAddress(podFQDN string, port int32) string {
	return joinHostPort(podFQDN, port)
}

func getHeadlessConnAddresses(connAddress, componentName string, replicas int32, isHeadless bool) []string {
	if isHeadless {
		addresses := make([]string, 0, replicas)
		for i := int32(0); i < replicas; i++ {
			addresses = append(addresses, fmt.Sprintf("%s.%s", getPodName(componentName, i), connAddress))
		}
		return addresses
	}
	return []string{connAddress}
}

func getKubernetesClusterDomain() string {
	if domain := os.Getenv("KUBERNETES_CLUSTER_DOMAIN"); len(domain) > 0 {
		return domain
	}
	return "cluster.local"
}

func joinHostPort(host string, port int32) string {
	if strings.IndexByte(host, ':') >= 0 {
		return fmt.Sprintf("[%s]:%d", host, port)
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func upgradeStatefulSet(sts *appsv1.StatefulSet) (*kruisev1alpha1.StatefulSet, error) {
	data, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	newSts := &kruisev1alpha1.StatefulSet{}
	if err := json.Unmarshal(data, newSts); err != nil {
		return nil, err
	}

	newSts.TypeMeta.APIVersion = kruisev1alpha1.SchemeGroupVersion.String()
	newSts.Spec.Template.Spec.ReadinessGates = []corev1.PodReadinessGate{
		{
			ConditionType: kruisepub.InPlaceUpdateReady,
		},
	}
	newSts.Spec.UpdateStrategy.RollingUpdate.PodUpdatePolicy = kruisev1alpha1.InPlaceIfPossiblePodUpdateStrategyType

	return newSts, nil
}

func getResources(res *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	if res == nil {
		return nil
	}
	res = res.DeepCopy()

	if res.Limits != nil {
		delete(res.Limits, corev1.ResourceStorage)
	}
	if res.Requests != nil {
		delete(res.Requests, corev1.ResourceStorage)
	}

	return res
}

func getConfigKey(componentType string) string {
	return fmt.Sprintf("nebula-%s.conf", componentType)
}

func parseStorageRequest(res corev1.ResourceList) (corev1.ResourceRequirements, error) {
	if res == nil {
		return corev1.ResourceRequirements{}, nil
	}
	storage, ok := res[corev1.ResourceStorage]
	if !ok {
		return corev1.ResourceRequirements{}, fmt.Errorf("storage request is not set")
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: storage,
		},
	}, nil
}

func generateContainers(c NebulaClusterComponentter, cm *corev1.ConfigMap) []corev1.Container {
	componentType := c.Type().String()
	nc := c.GetNebulaCluster()

	containers := make([]corev1.Container, 0, 1)

	metadAddress := strings.Join(nc.GetMetadEndpoints(), ",")
	cmd := []string{"/bin/bash", "-ecx"}
	cmd = append(cmd, fmt.Sprintf("exec /usr/local/nebula/bin/nebula-%s", componentType)+
		fmt.Sprintf(" --flagfile=/usr/local/nebula/etc/nebula-%s.conf", componentType)+
		" --meta_server_addrs="+metadAddress+
		" --local_ip=$(hostname)."+c.GetServiceFQDN()+
		" --ws_ip=$(hostname)."+c.GetServiceFQDN()+
		" --minloglevel=1"+
		" --v=0"+
		" --daemonize=false")

	mounts := c.GenerateVolumeMounts()
	if cm != nil {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      c.GetName(),
			MountPath: "/usr/local/nebula/etc",
		})
	}

	ports := c.GenerateContainerPorts()

	container := corev1.Container{
		Name:         componentType,
		Image:        c.GetImage(),
		Command:      cmd,
		Env:          c.GetPodEnvVars(),
		Ports:        ports,
		VolumeMounts: mounts,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/status",
					Port:   intstr.FromInt(int(ports[1].ContainerPort)),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: int32(20),
			TimeoutSeconds:      int32(5),
			PeriodSeconds:       int32(10),
		},
	}

	resources := c.GetResources()
	if resources != nil {
		container.Resources = *resources
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}

	containers = append(containers, container)

	return containers
}

func generateStatefulSet(c NebulaClusterComponentter, cm *corev1.ConfigMap, enableEvenPodsSpread bool) (*appsv1.StatefulSet, error) {
	namespace := c.GetNamespace()
	svcName := c.GetServiceName()
	componentType := c.Type().String()
	componentLabel := c.GenerateLabels()

	nc := c.GetNebulaCluster()

	configKey := getConfigKey(componentType)
	containers := generateContainers(c, cm)
	volumes := c.GenerateVolumes()
	if cm != nil {
		volumes = append(volumes, corev1.Volume{
			Name: c.GetName(),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: c.GetName(),
					},
					Items: []corev1.KeyToPath{{
						Key:  configKey,
						Path: configKey,
					}},
				},
			},
		})
	}

	podSpec := corev1.PodSpec{
		SchedulerName:    nc.Spec.SchedulerName,
		NodeSelector:     nc.Spec.NodeSelector,
		Containers:       containers,
		Volumes:          volumes,
		ImagePullSecrets: nc.Spec.ImagePullSecrets,
	}

	if nc.Spec.SchedulerName == corev1.DefaultSchedulerName && enableEvenPodsSpread {
		podSpec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           int32(1),
				TopologyKey:       label.NodeHostnameLabelKey,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector:     label.Label(componentLabel).LabelSelector(),
			},
		}
	}

	scName, storageRes := c.GetStorageClass(), c.GetStorageResources()
	storageRequest, err := parseStorageRequest(storageRes.Requests)
	if err != nil {
		return nil, fmt.Errorf("cannot parse storage request for %s, error: %v", err, componentType)
	}

	replicas := c.GetReplicas()
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.GetName(),
			Namespace:       namespace,
			Labels:          componentLabel,
			OwnerReferences: c.GenerateOwnerReferences(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: componentLabel},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mergeStringMaps(false, componentLabel, c.GetPodLabels()),
					Annotations: c.GetPodAnnotations(),
				},
				Spec: podSpec,
			},
			ServiceName:         svcName,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: &replicas,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: componentType,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources:        storageRequest,
						StorageClassName: scName,
					},
				},
			},
		},
	}

	return sts, nil
}

func convertToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: objMap,
	}, nil
}

func generateWorkload(
	c NebulaClusterComponentter,
	gvk schema.GroupVersionKind,
	cm *corev1.ConfigMap,
	enableEvenPodsSpread bool,
) (*unstructured.Unstructured, error) {
	var w interface{}
	var err error

	switch gvk.String() {
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").String():
		w, err = generateStatefulSet(c, cm, enableEvenPodsSpread)
		if err != nil {
			return nil, err
		}
	case kruisev1alpha1.SchemeGroupVersion.WithKind("StatefulSet").String():
		var set *appsv1.StatefulSet
		set, err = generateStatefulSet(c, cm, enableEvenPodsSpread)
		if err != nil {
			return nil, err
		}
		w, err = upgradeStatefulSet(set)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported kind %s", gvk.String())
	}
	u, err := convertToUnstructured(w)
	if err != nil {
		return nil, err
	}
	u.SetAPIVersion(gvk.GroupVersion().String())
	u.SetKind(gvk.Kind)
	return u, err
}

func generateService(c NebulaClusterComponentter) *corev1.Service {
	namespace := c.GetNamespace()
	svcName := c.GetServiceName()

	var ports []corev1.ServicePort
	for _, port := range c.GenerateContainerPorts() {
		ports = append(ports, corev1.ServicePort{
			Name:       port.Name,
			Protocol:   corev1.ProtocolTCP,
			Port:       port.ContainerPort,
			TargetPort: intstr.FromInt(int(port.ContainerPort)),
		})
	}

	selector := c.GenerateLabels()
	labels := label.Label(selector).Copy().Labels()

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       namespace,
			OwnerReferences: c.GenerateOwnerReferences(),
			Labels:          labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    ports,
		},
	}

	if c.IsHeadlessService() {
		service.Spec.ClusterIP = corev1.ClusterIPNone
		service.Spec.PublishNotReadyAddresses = true
	} else {
		serviceSpec := c.GetServiceSpec()
		if serviceSpec != nil && serviceSpec.ClusterIP != nil {
			service.Spec.ClusterIP = *serviceSpec.ClusterIP
		}
	}

	return service
}

func generateConfigMap(c NebulaClusterComponentter) *corev1.ConfigMap {
	namespace := c.GetNamespace()
	labels := c.GenerateLabels()
	cmName := c.GetName()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cmName,
			Namespace:       namespace,
			OwnerReferences: c.GenerateOwnerReferences(),
			Labels:          labels,
		},
	}
	cm.Data = make(map[string]string)

	return cm
}

func mergeStringMaps(overwrite bool, ms ...map[string]string) map[string]string {
	n := 0
	for _, m := range ms {
		n += len(m)
	}
	mp := make(map[string]string, n)
	if n == 0 {
		return mp
	}
	for _, m := range ms {
		for k, v := range m {
			if overwrite || !isStringMapExist(mp, k) {
				mp[k] = v
			}
		}
	}
	return mp
}

func isStringMapExist(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, exist := m[key]
	return exist
}
