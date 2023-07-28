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
	"strconv"
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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
)

const (
	AgentSidecarContainerName = "ng-agent"
	AgentInitContainerName    = "ng-init-agent"
	DefaultAgentPortGRPC      = 8888
	agentPortNameGRPC         = "grpc"
	defaultAgentImage         = "vesoft/nebula-agent:latest"
)

func getComponentName(clusterName string, typ ComponentType) string {
	return fmt.Sprintf("%s-%s", clusterName, typ)
}

func getPodName(componentName string, ordinal int32) string {
	return fmt.Sprintf("%s-%d", componentName, ordinal)
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

func getConnAddresses(connAddress, componentName string, replicas int32) []string {
	addresses := make([]string, 0, replicas)
	for i := int32(0); i < replicas; i++ {
		addresses = append(addresses, fmt.Sprintf("%s.%s", getPodName(componentName, i), connAddress))
	}
	return addresses
}

func getKubernetesClusterDomain() string {
	if domain := os.Getenv("KUBERNETES_CLUSTER_DOMAIN"); len(domain) > 0 {
		return domain
	}
	return "cluster.local"
}

func joinHostPort(host string, port int32) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func getTopologySpreadConstraints(constraints []TopologySpreadConstraint, labels map[string]string) []corev1.TopologySpreadConstraint {
	if len(constraints) == 0 {
		return nil
	}

	tscs := make([]corev1.TopologySpreadConstraint, 0, len(constraints))
	for _, constraint := range constraints {
		tsc := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       constraint.TopologyKey,
			WhenUnsatisfiable: constraint.WhenUnsatisfiable,
		}
		tsc.LabelSelector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		tscs = append(tscs, tsc)
	}
	return tscs
}

func getClientCertsVolume(sslCerts *SSLCertsSpec) []corev1.Volume {
	if sslCerts == nil {
		return nil
	}

	return []corev1.Volume{
		{
			Name: "client-crt",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sslCerts.ClientSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  sslCerts.ClientCert,
							Path: "client.crt",
						},
					},
				},
			},
		},
		{
			Name: "client-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sslCerts.ClientSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  sslCerts.ClientKey,
							Path: "client.key",
						},
					},
				},
			},
		},
		{
			Name: "client-ca-crt",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sslCerts.CASecret,
					Items: []corev1.KeyToPath{
						{
							Key:  sslCerts.CACert,
							Path: "ca.crt",
						},
					},
				},
			},
		},
	}
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

func getCmKey(componentType string) string {
	return fmt.Sprintf("nebula-%s.conf", componentType)
}

func logVolume(componentType string) string {
	return componentType + "-log"
}

func dataVolume(componentType string) string {
	return componentType + "-data"
}

func storageDataVolume(componentType string, index int) string {
	if index > 0 {
		return fmt.Sprintf("%s-data-%d", componentType, index)
	}
	return dataVolume(componentType)
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

func GenerateInitAgentContainer(c NebulaClusterComponent) corev1.Container {
	container := generateAgentContainer(c, true)
	container.Name = AgentInitContainerName

	return container
}

func generateAgentContainer(c NebulaClusterComponent, init bool) corev1.Container {
	nc := c.GetNebulaCluster()
	componentType := c.ComponentType().String()
	metadAddr := nc.GetMetadThriftConnAddress()

	cmd := []string{"/bin/sh", "-ecx"}
	initCmd := "sleep 30; exec /usr/local/bin/agent" +
		fmt.Sprintf(" --agent=$(hostname).%s:%d", c.GetServiceFQDN(), DefaultAgentPortGRPC) +
		" --ratelimit=1073741824 --debug"
	brCmd := initCmd + " --meta=" + metadAddr
	logCmd := "sh /logrotate.sh; /etc/init.d/cron start"
	logfgCmd := "sh /logrotate.sh; exec cron -f"

	if nc.IsMetadSSLEnabled() || nc.IsClusterSSLEnabled() {
		initCmd += " --enable_ssl"
		brCmd += " --enable_ssl"
		if nc.InsecureSkipVerify() {
			initCmd += " --insecure_skip_verify"
			brCmd += " --insecure_skip_verify"
		}
	}

	if init {
		cmd = append(cmd, initCmd)
	} else {
		if nc.IsLogRotateEnabled() && nc.IsBREnabled() {
			cmd = append(cmd, fmt.Sprintf(`%s; %s`, logCmd, brCmd))
		} else if nc.IsLogRotateEnabled() {
			cmd = append(cmd, logfgCmd)
		} else if nc.IsBREnabled() {
			cmd = append(cmd, brCmd)
		}
	}

	agentImage := defaultAgentImage
	if nc.Spec.Agent.Image != "" {
		agentImage = nc.Spec.Agent.Image
	}
	if nc.Spec.Agent.Version != "" {
		agentImage = fmt.Sprintf("%s:%s", agentImage, nc.Spec.Agent.Version)
	}
	container := corev1.Container{
		Name:      AgentSidecarContainerName,
		Image:     agentImage,
		Command:   cmd,
		Resources: nc.Spec.Agent.Resources,
	}
	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}

	if nc.IsBREnabled() {
		if c.ComponentType() != GraphdComponentType {
			container.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      dataVolume(componentType),
					MountPath: "/usr/local/nebula/data",
					SubPath:   "data",
				},
			}
		}

		container.Ports = []corev1.ContainerPort{
			{
				Name:          agentPortNameGRPC,
				ContainerPort: int32(DefaultAgentPortGRPC),
			},
		}
	}

	if nc.IsLogRotateEnabled() {
		logRotate := nc.Spec.LogRotate
		container.Env = []corev1.EnvVar{
			{
				Name:  "LOGROTATE_ROTATE",
				Value: strconv.Itoa(int(logRotate.Rotate)),
			},
			{
				Name:  "LOGROTATE_SIZE",
				Value: logRotate.Size,
			},
		}

		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      logVolume(componentType),
			MountPath: "/usr/local/nebula/logs",
			SubPath:   "logs",
		})
	}

	if (nc.IsMetadSSLEnabled() || nc.IsClusterSSLEnabled()) && nc.IsBREnabled() {
		certMounts := []corev1.VolumeMount{
			{
				Name:      "client-crt",
				ReadOnly:  true,
				MountPath: "/usr/local/certs/client.crt",
				SubPath:   "client.crt",
			},
			{
				Name:      "client-key",
				ReadOnly:  true,
				MountPath: "/usr/local/certs/client.key",
				SubPath:   "client.key",
			},
			{
				Name:      "client-ca-crt",
				ReadOnly:  true,
				MountPath: "/usr/local/certs/ca.crt",
				SubPath:   "ca.crt",
			},
		}
		container.VolumeMounts = append(container.VolumeMounts, certMounts...)
	}

	return container
}

func generateContainers(c NebulaClusterComponent, cm *corev1.ConfigMap) []corev1.Container {
	componentType := c.ComponentType().String()
	nc := c.GetNebulaCluster()

	containers := make([]corev1.Container, 0, 1)

	cmd := []string{"/bin/bash", "-ecx"}

	var dataPath string
	volumes := len(nc.Spec.Storaged.DataVolumeClaims)
	if c.ComponentType() == StoragedComponentType && volumes > 1 {
		dataPath = " --data_path=data/storage"
		for i := 1; i < volumes; i++ {
			dataPath += fmt.Sprintf(",data%d/storage", i)
		}
	}

	metadAddress := strings.Join(nc.GetMetadEndpoints(MetadPortNameThrift), ",")
	flags := " --meta_server_addrs=" + metadAddress +
		" --local_ip=$(hostname)." + c.GetServiceFQDN() +
		" --ws_ip=$(hostname)." + c.GetServiceFQDN() +
		" --daemonize=false" + dataPath
	if c.ComponentType() == MetadComponentType && nc.Spec.Metad.LicenseManagerURL != nil {
		flags += " --license_manager_url=" + pointer.StringDeref(nc.Spec.Metad.LicenseManagerURL, "")
	}

	cmd = append(cmd, fmt.Sprintf("exec /usr/local/nebula/bin/nebula-%s", componentType)+
		fmt.Sprintf(" --flagfile=/usr/local/nebula/etc/nebula-%s.conf", componentType)+flags)

	mounts := c.GenerateVolumeMounts()
	if cm != nil {
		subPath := getCmKey(c.ComponentType().String())
		mounts = append(mounts, corev1.VolumeMount{
			Name:      c.GetName(),
			MountPath: fmt.Sprintf("/usr/local/nebula/etc/%s", subPath),
			SubPath:   subPath,
		})
	}

	ports := c.GenerateContainerPorts()

	baseContainer := corev1.Container{
		Name:         componentType,
		Image:        c.ComponentSpec().PodImage(),
		Command:      cmd,
		Env:          c.ComponentSpec().PodEnvVars(),
		Ports:        ports,
		VolumeMounts: mounts,
	}

	baseContainer.LivenessProbe = c.ComponentSpec().LivenessProbe()
	readinessProbe := c.ComponentSpec().ReadinessProbe()
	if readinessProbe != nil {
		baseContainer.ReadinessProbe = readinessProbe
	} else {
		baseContainer.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/status",
					Port:   intstr.FromInt(int(ports[1].ContainerPort)),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: int32(10),
			TimeoutSeconds:      int32(5),
			PeriodSeconds:       int32(10),
		}
	}

	resources := c.ComponentSpec().Resources()
	if resources != nil {
		baseContainer.Resources = *resources
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		baseContainer.ImagePullPolicy = *imagePullPolicy
	}

	containers = append(containers, baseContainer)

	if nc.IsBREnabled() || nc.IsLogRotateEnabled() {
		agentContainer := generateAgentContainer(c, false)
		containers = append(containers, agentContainer)
	}

	containers = mergeSidecarContainers(containers, c.ComponentSpec().SidecarContainers())

	return containers
}

func generateStatefulSet(c NebulaClusterComponent, cm *corev1.ConfigMap) (*appsv1.StatefulSet, error) {
	namespace := c.GetNamespace()
	svcName := c.GetServiceName()
	componentType := c.ComponentType().String()
	componentLabel := c.GenerateLabels()

	nc := c.GetNebulaCluster()

	cmKey := getCmKey(componentType)
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
						Key:  cmKey,
						Path: cmKey,
					}},
				},
			},
		})
	}
	if (nc.IsMetadSSLEnabled() || nc.IsClusterSSLEnabled()) && nc.IsBREnabled() {
		certVolumes := getClientCertsVolume(nc.Spec.SSLCerts)
		volumes = append(volumes, certVolumes...)
	}

	volumes = mergeVolumes(volumes, c.ComponentSpec().SidecarVolumes())

	podSpec := corev1.PodSpec{
		SchedulerName:    nc.Spec.SchedulerName,
		NodeSelector:     c.ComponentSpec().NodeSelector(),
		InitContainers:   c.ComponentSpec().InitContainers(),
		Containers:       containers,
		Volumes:          volumes,
		ImagePullSecrets: nc.Spec.ImagePullSecrets,
		Affinity:         c.ComponentSpec().Affinity(),
		Tolerations:      c.ComponentSpec().Tolerations(),
	}

	if nc.Spec.SchedulerName == corev1.DefaultSchedulerName {
		podSpec.TopologySpreadConstraints = getTopologySpreadConstraints(nc.Spec.TopologySpreadConstraints, componentLabel)
	}

	volumeClaim, err := c.GenerateVolumeClaim()
	if err != nil {
		return nil, err
	}

	apply, err := json.Marshal(c.GetConfig())
	if err != nil {
		return nil, err
	}

	mergeLabels := mergeStringMaps(true, componentLabel, c.ComponentSpec().PodLabels())
	replicas := c.ComponentSpec().Replicas()
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.GetName(),
			Namespace:       namespace,
			Annotations:     map[string]string{annotation.AnnLastAppliedFlagsKey: string(apply)},
			Labels:          componentLabel,
			OwnerReferences: c.GenerateOwnerReferences(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: mergeLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mergeLabels,
					Annotations: c.ComponentSpec().PodAnnotations(),
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
			VolumeClaimTemplates: volumeClaim,
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
	c NebulaClusterComponent,
	gvk schema.GroupVersionKind,
	cm *corev1.ConfigMap,
) (*unstructured.Unstructured, error) {
	var w interface{}
	var err error

	switch gvk.String() {
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").String():
		w, err = generateStatefulSet(c, cm)
		if err != nil {
			return nil, err
		}
	case kruisev1alpha1.SchemeGroupVersion.WithKind("StatefulSet").String():
		var set *appsv1.StatefulSet
		set, err = generateStatefulSet(c, cm)
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

func generateService(c NebulaClusterComponent) *corev1.Service {
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

	serviceSpec := c.GetServiceSpec()
	if serviceSpec != nil && len(serviceSpec.Annotations) > 0 {
		service.Annotations = serviceSpec.Annotations
	}

	if c.IsHeadlessService() {
		service.Spec.ClusterIP = corev1.ClusterIPNone
		service.Spec.PublishNotReadyAddresses = true
	} else if serviceSpec != nil {
		if serviceSpec.ClusterIP != nil {
			service.Spec.ClusterIP = *serviceSpec.ClusterIP
		}
		if serviceSpec.Type != "" {
			service.Spec.Type = serviceSpec.Type
		}
	}

	return service
}

func generateConfigMap(c NebulaClusterComponent) *corev1.ConfigMap {
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

func mergeVolumes(original []corev1.Volume, additional []corev1.Volume) []corev1.Volume {
	exists := sets.NewString()
	for _, volume := range original {
		exists.Insert(volume.Name)
	}

	for _, volume := range additional {
		if exists.Has(volume.Name) {
			continue
		}
		original = append(original, volume)
		exists.Insert(volume.Name)
	}

	return original
}

func mergeSidecarContainers(origins, injected []corev1.Container) []corev1.Container {
	containersInPod := make(map[string]int)
	for index, container := range origins {
		containersInPod[container.Name] = index
	}

	var appContainers []corev1.Container
	for _, sidecar := range injected {
		if index, ok := containersInPod[sidecar.Name]; ok {
			origins[index] = sidecar
			continue
		}
		appContainers = append(appContainers, sidecar)
	}

	origins = append(origins, appContainers...)
	return origins
}
