package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
)

func (nc *NebulaCluster) GetExporterImage() string {
	return getImage(nc.Spec.Exporter.Image, nc.Spec.Exporter.Version, defaultExporterImage)
}

func (nc *NebulaCluster) GetExporterReplicas() *int32 {
	return nc.Spec.Exporter.Replicas
}

func (nc *NebulaCluster) GetExporterResources() *corev1.ResourceRequirements {
	return getResources(nc.Spec.Exporter.Resources)
}

func (nc *NebulaCluster) GetExporterEnvVars() []corev1.EnvVar {
	return nc.Spec.Exporter.PodSpec.EnvVars
}

func (nc *NebulaCluster) GetExporterPodAnnotations() map[string]string {
	return nc.Spec.Exporter.PodSpec.Annotations
}

func (nc *NebulaCluster) GetExporterLabels() map[string]string {
	selector := label.New().Cluster(nc.GetName()).Exporter()
	labels := selector.Copy().Labels()
	podLabels := nc.Spec.Exporter.PodSpec.Labels

	return mergeStringMaps(true, labels, podLabels)
}

func (nc *NebulaCluster) GetExporterNodeSelector() map[string]string {
	selector := map[string]string{}
	for k, v := range nc.Spec.NodeSelector {
		selector[k] = v
	}
	for k, v := range nc.Spec.Exporter.PodSpec.NodeSelector {
		selector[k] = v
	}
	return selector
}

func (nc *NebulaCluster) GetExporterAffinity() *corev1.Affinity {
	affinity := nc.Spec.Graphd.PodSpec.Affinity
	if affinity == nil {
		affinity = nc.Spec.Affinity
	}
	return affinity
}

func (nc *NebulaCluster) GetExporterTolerations() []corev1.Toleration {
	tolerations := nc.Spec.Exporter.PodSpec.Tolerations
	if len(tolerations) == 0 {
		return nc.Spec.Tolerations
	}
	return tolerations
}

func (nc *NebulaCluster) GetExporterLivenessProbe() *corev1.Probe {
	return nc.Spec.Exporter.PodSpec.LivenessProbe
}
