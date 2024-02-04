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

	kruisev1beta1 "github.com/openkruise/kruise-api/apps/v1beta1"
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
	NebulaServiceAccountName  = "nebula-sa"
	NebulaRoleName            = "nebula-role"
	NebulaRoleBindingName     = "nebula-rolebinding"
	LogSidecarContainerName   = "ng-logrotate"
	AgentSidecarContainerName = "ng-agent"
	AgentInitContainerName    = "ng-init-agent"
	DefaultAgentPortGRPC      = 8888
	AgentPortNameGRPC         = "grpc"
	DefaultAgentImage         = "vesoft/nebula-agent"
	DefaultAlpineImage        = "vesoft/nebula-alpine:latest"

	ZoneSuffix = "zone"
)

func GetClientCertVolumes(sslCerts *SSLCertsSpec) []corev1.Volume {
	return getClientCertVolumes(sslCerts)
}

func GetClientCertVolumeMounts() []corev1.VolumeMount {
	return getClientCertVolumeMounts()
}

func GenerateInitAgentContainer(c NebulaClusterComponent) corev1.Container {
	container := generateAgentContainer(c, true)
	container.Name = AgentInitContainerName

	return container
}

func EnableLocalCerts() bool {
	return os.Getenv("CA_CERT_PATH") != "" &&
		os.Getenv("CLIENT_CERT_PATH") != "" &&
		os.Getenv("CLIENT_KEY_PATH") != ""
}

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

func parseCustomPort(defaultPort int32, customPort string) (int32, error) {
	p := defaultPort
	if customPort != "" {
		v, err := strconv.Atoi(customPort)
		if err != nil {
			return 0, err
		}
		p = int32(v)
	}
	return p, nil
}

func getStoragedDataVolumeMounts(c NebulaClusterComponent) []corev1.VolumeMount {
	nc := c.GetNebulaCluster()
	componentType := c.ComponentType().String()
	mounts := make([]corev1.VolumeMount, 0)
	for i := range nc.Spec.Storaged.DataVolumeClaims {
		volumeName := storageDataVolume(componentType, i)
		mountPath := "/usr/local/nebula/data"
		subPath := "data"
		if i > 0 {
			mountPath = fmt.Sprintf("/usr/local/nebula/data%d", i)
			subPath = fmt.Sprintf("data%d", i)
		}
		mount := corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
			SubPath:   subPath,
		}
		mounts = append(mounts, mount)
	}
	return mounts
}

func getClientCertVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
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
}

func getClientCertVolumes(sslCerts *SSLCertsSpec) []corev1.Volume {
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

func rollingUpdateDone(workloadStatus *WorkloadStatus) bool {
	return workloadStatus.UpdatedReplicas == workloadStatus.Replicas &&
		workloadStatus.ReadyReplicas == workloadStatus.Replicas &&
		workloadStatus.CurrentReplicas == workloadStatus.UpdatedReplicas &&
		workloadStatus.CurrentRevision == workloadStatus.UpdateRevision
}

func upgradeStatefulSet(sts *appsv1.StatefulSet) (*kruisev1beta1.StatefulSet, error) {
	data, err := json.Marshal(sts)
	if err != nil {
		return nil, err
	}
	newSts := &kruisev1beta1.StatefulSet{}
	if err := json.Unmarshal(data, newSts); err != nil {
		return nil, err
	}

	newSts.TypeMeta.APIVersion = kruisev1beta1.SchemeGroupVersion.String()
	//newSts.Spec.Template.Spec.ReadinessGates = []corev1.PodReadinessGate{
	//	{
	//		ConditionType: kruisepub.InPlaceUpdateReady,
	//	},
	//}
	newSts.Spec.UpdateStrategy.RollingUpdate.PodUpdatePolicy = kruisev1beta1.RecreatePodUpdateStrategyType

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

func logVolumeExists(componentType string, volumes []corev1.Volume) bool {
	logVolName := logVolume(componentType)
	for _, volume := range volumes {
		if volume.Name == logVolName {
			return true
		}
	}
	return false
}

func generateLogContainer(c NebulaClusterComponent) corev1.Container {
	nc := c.GetNebulaCluster()
	componentType := c.ComponentType().String()

	image := DefaultAlpineImage
	if nc.Spec.AlpineImage != nil {
		image = pointer.StringDeref(nc.Spec.AlpineImage, "")
	}

	cmd := []string{"/bin/sh", "-ecx", "sh /logrotate.sh; crond -f -l 2"}
	container := corev1.Container{
		Name:    LogSidecarContainerName,
		Image:   image,
		Command: cmd,
	}

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
		cmd = append(cmd, brCmd)
	}

	container := corev1.Container{
		Name:    AgentSidecarContainerName,
		Image:   DefaultAgentImage,
		Command: cmd,
	}
	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}
	if nc.Spec.Agent != nil {
		var agentImage string
		if nc.Spec.Agent.Image != "" {
			agentImage = nc.Spec.Agent.Image
		}
		if nc.Spec.Agent.Version != "" {
			agentImage = fmt.Sprintf("%s:%s", agentImage, nc.Spec.Agent.Version)
		}
		container.Image = agentImage
		container.Resources = nc.Spec.Agent.Resources
	}

	if nc.IsBREnabled() {
		if c.ComponentType() == MetadComponentType {
			container.VolumeMounts = []corev1.VolumeMount{
				{
					Name:      dataVolume(componentType),
					MountPath: "/usr/local/nebula/data",
					SubPath:   "data",
				},
			}
		} else if c.ComponentType() == StoragedComponentType {
			container.VolumeMounts = getStoragedDataVolumeMounts(c)
		}

		container.Ports = []corev1.ContainerPort{
			{
				Name:          AgentPortNameGRPC,
				ContainerPort: int32(DefaultAgentPortGRPC),
			},
		}
	}

	if (nc.IsMetadSSLEnabled() || nc.IsClusterSSLEnabled()) && nc.IsBREnabled() && !EnableLocalCerts() {
		certMounts := getClientCertVolumeMounts()
		container.VolumeMounts = append(container.VolumeMounts, certMounts...)
	}

	return container
}

func genDynamicFlagsContainer(c NebulaClusterComponent) corev1.Container {
	nc := c.GetNebulaCluster()
	script := `
set -exo pipefail

TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt
            
curl -s --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X GET ${APISERVER}/apis/apps/v1/namespaces/${NAMESPACE}/statefulsets/${PARENT_NAME} | jq .metadata.annotations > /metadata/annotations.json
jq '."nebula-graph.io/last-applied-dynamic-flags" | fromjson' /metadata/annotations.json > /metadata/flags.json
cat /metadata/flags.json
`
	image := DefaultAlpineImage
	if nc.Spec.AlpineImage != nil {
		image = pointer.StringDeref(nc.Spec.AlpineImage, "")
	}

	container := corev1.Container{
		Name:    "dynamic-flags",
		Image:   image,
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{`echo "$SCRIPT" > /tmp/dynamic-flags-script && sh /tmp/dynamic-flags-script`},
		Env: []corev1.EnvVar{
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name:  "PARENT_NAME",
				Value: c.GetName(),
			},
			{
				Name:  "APISERVER",
				Value: "https://kubernetes.default.svc",
			},
			{
				Name:  "SERVICEACCOUNT",
				Value: "/var/run/secrets/kubernetes.io/serviceaccount",
			},
			{
				Name:  "SCRIPT",
				Value: script,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "flags",
				MountPath: "/metadata",
			},
		},
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}
	return container
}

func genNodeLabelsContainer(nc *NebulaCluster) corev1.Container {
	script := `
set -exo pipefail

TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt
            
curl -s --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X GET ${APISERVER}/api/v1/nodes/${NODENAME} | jq .metadata.labels > /node/labels.json

NODE_ZONE=$(jq '."topology.kubernetes.io/zone"' -r /node/labels.json)
echo "NODE_ZONE is ${NODE_ZONE}"
echo "export NODE_ZONE=${NODE_ZONE}" > /node/zone
`
	image := DefaultAlpineImage
	if nc.Spec.AlpineImage != nil {
		image = pointer.StringDeref(nc.Spec.AlpineImage, "")
	}

	container := corev1.Container{
		Name:    "node-labels",
		Image:   image,
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{`echo "$SCRIPT" > /tmp/node-zone-script && sh /tmp/node-zone-script`},
		Env: []corev1.EnvVar{
			{
				Name: "NODENAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
			{
				Name:  "APISERVER",
				Value: "https://kubernetes.default.svc",
			},
			{
				Name:  "SERVICEACCOUNT",
				Value: "/var/run/secrets/kubernetes.io/serviceaccount",
			},
			{
				Name:  "SCRIPT",
				Value: script,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "node-info",
				MountPath: "/node",
			},
		},
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}
	return container
}

func generateInitContainers(c NebulaClusterComponent) []corev1.Container {
	containers := c.ComponentSpec().InitContainers()
	nc := c.GetNebulaCluster()
	if c.ComponentType() == GraphdComponentType && nc.IsZoneEnabled() {
		nodeLabelsContainer := genNodeLabelsContainer(nc)
		containers = append(containers, nodeLabelsContainer)
	}
	containers = append(containers, genDynamicFlagsContainer(c))
	return containers
}

func generateNebulaContainers(c NebulaClusterComponent, cm *corev1.ConfigMap, dynamicFlags map[string]string) ([]corev1.Container, error) {
	componentType := c.ComponentType().String()
	nc := c.GetNebulaCluster()

	containers := make([]corev1.Container, 0, 1)

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
		" --daemonize=false" + dataPath
	if c.ComponentType() == MetadComponentType && nc.Spec.Metad.LicenseManagerURL != nil {
		flags += " --license_manager_url=" + pointer.StringDeref(nc.Spec.Metad.LicenseManagerURL, "")
	}
	if c.ComponentType() == GraphdComponentType && nc.IsZoneEnabled() {
		flags += " --assigned_zone=$NODE_ZONE"
	}

	cmd := []string{"/bin/sh", "-ecx"}
	if c.ComponentType() == GraphdComponentType && nc.IsZoneEnabled() {
		cmd = append(cmd, fmt.Sprintf(". /node/zone; echo $NODE_ZONE; exec /usr/local/nebula/bin/nebula-%s", componentType)+
			fmt.Sprintf(" --flagfile=/usr/local/nebula/etc/nebula-%s.conf", componentType)+flags)
	} else {
		cmd = append(cmd, fmt.Sprintf("exec /usr/local/nebula/bin/nebula-%s", componentType)+
			fmt.Sprintf(" --flagfile=/usr/local/nebula/etc/nebula-%s.conf", componentType)+flags)
	}

	mounts := c.GenerateVolumeMounts()
	if cm != nil {
		subPath := getCmKey(c.ComponentType().String())
		mounts = append(mounts, corev1.VolumeMount{
			Name:      c.GetName(),
			MountPath: fmt.Sprintf("/usr/local/nebula/etc/%s", subPath),
			SubPath:   subPath,
		})
		mounts = append(mounts, c.ComponentSpec().VolumeMounts()...)
	}
	if c.ComponentType() == GraphdComponentType && nc.IsZoneEnabled() {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "node-info",
			MountPath: "/node",
		})
	}
	mounts = append(mounts, corev1.VolumeMount{
		Name:      "flags",
		MountPath: "/metadata",
	})

	ports := c.GenerateContainerPorts()

	baseContainer := corev1.Container{
		Name:            componentType,
		Image:           c.ComponentSpec().PodImage(),
		Command:         cmd,
		Env:             c.ComponentSpec().PodEnvVars(),
		SecurityContext: c.ComponentSpec().SecurityContext(),
		Ports:           ports,
		VolumeMounts:    mounts,
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

	script := `
set -x

if [ ! -s "metadata/flags.json" ]; then
 echo "flags.json is empty"
 exit 0
fi
while :
do
  curl -i -X PUT -H "Content-Type: application/json" -d @/metadata/flags.json -s "http://${MY_IP}:${HTTP_PORT}/flags"
  if [ $? -eq 0 ]
  then
    break
  fi
  sleep 1
done
`

	if len(dynamicFlags) > 0 {
		envVars := []corev1.EnvVar{
			{
				Name: "MY_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
			{
				Name:  "HTTP_PORT",
				Value: strconv.Itoa(int(ports[1].ContainerPort)),
			},
			{
				Name:  "SCRIPT",
				Value: script,
			},
		}
		baseContainer.Env = append(baseContainer.Env, envVars...)
		baseContainer.Lifecycle = &corev1.Lifecycle{
			PostStart: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", `echo "$SCRIPT" > /tmp/post-start-script && sh /tmp/post-start-script`},
				},
			},
		}
	}

	containers = append(containers, baseContainer)

	if nc.IsBREnabled() {
		agentContainer := generateAgentContainer(c, false)
		containers = append(containers, agentContainer)
	}
	if nc.IsLogRotateEnabled() && logVolumeExists(componentType, c.GenerateVolumes()) {
		logContainer := generateLogContainer(c)
		containers = append(containers, logContainer)
	}

	containers = mergeSidecarContainers(containers, c.ComponentSpec().SidecarContainers())

	return containers, nil
}

func generateStatefulSet(c NebulaClusterComponent, cm *corev1.ConfigMap) (*appsv1.StatefulSet, error) {
	namespace := c.GetNamespace()
	svcName := c.GetHeadlessServiceName()
	componentType := c.ComponentType().String()
	componentLabel := c.GenerateLabels()

	nc := c.GetNebulaCluster()

	dynamicFlags, _ := separateFlags(c.GetConfig())

	cmKey := getCmKey(componentType)
	initContainers := generateInitContainers(c)
	containers, err := generateNebulaContainers(c, cm, dynamicFlags)
	if err != nil {
		return nil, err
	}
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
	if (nc.IsMetadSSLEnabled() || nc.IsClusterSSLEnabled()) && nc.IsBREnabled() && !EnableLocalCerts() {
		certVolumes := getClientCertVolumes(nc.Spec.SSLCerts)
		volumes = append(volumes, certVolumes...)
	}

	if c.ComponentType() == GraphdComponentType && nc.IsZoneEnabled() {
		volumes = append(volumes, corev1.Volume{
			Name: "node-info",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})
	}

	volumes = append(volumes, corev1.Volume{
		Name: "flags",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	})

	volumes = mergeVolumes(volumes, c.ComponentSpec().Volumes())

	podSpec := corev1.PodSpec{
		SchedulerName:      nc.Spec.SchedulerName,
		NodeSelector:       c.ComponentSpec().NodeSelector(),
		InitContainers:     initContainers,
		Containers:         containers,
		Volumes:            volumes,
		ImagePullSecrets:   nc.Spec.ImagePullSecrets,
		Affinity:           c.ComponentSpec().Affinity(),
		Tolerations:        c.ComponentSpec().Tolerations(),
		ServiceAccountName: NebulaServiceAccountName,
	}

	podSpec.TopologySpreadConstraints = c.ComponentSpec().TopologySpreadConstraints(componentLabel)

	volumeClaim, err := c.GenerateVolumeClaim()
	if err != nil {
		return nil, err
	}

	dynamicVal, err := json.Marshal(dynamicFlags)
	if err != nil {
		return nil, err
	}

	mergeLabels := mergeStringMaps(true, componentLabel, c.ComponentSpec().PodLabels())
	replicas := c.ComponentSpec().Replicas()
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.GetName(),
			Namespace: namespace,
			Annotations: map[string]string{
				annotation.AnnLastAppliedDynamicFlagsKey: string(dynamicVal),
			},
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
	case kruisev1beta1.SchemeGroupVersion.WithKind("StatefulSet").String():
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

func generateService(c NebulaClusterComponent, isHeadless bool) *corev1.Service {
	namespace := c.GetNamespace()
	svcName := getServiceName(c.GetName(), isHeadless)

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

	if isHeadless {
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

func separateFlags(config map[string]string) (map[string]string, map[string]string) {
	dynamic := make(map[string]string)
	static := make(map[string]string)
	for k, v := range config {
		if _, ok := DynamicFlags[k]; ok {
			dynamic[k] = v
		} else {
			static[k] = v
		}
	}
	return dynamic, static
}
