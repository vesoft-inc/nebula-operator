package component

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/maputil"
)

const defaultMetricsPort = 9100

type nebulaExporter struct {
	clientSet kube.ClientSet
}

func NewNebulaExporter(clientSet kube.ClientSet) ReconcileManager {
	return &nebulaExporter{clientSet: clientSet}
}

func (e *nebulaExporter) Reconcile(nc *v1alpha1.NebulaCluster) error {
	if nc.Spec.Exporter == nil {
		return nil
	}

	if err := e.syncExporterService(nc); err != nil {
		return err
	}

	return e.syncExporterDeployment(nc)
}

func (e *nebulaExporter) syncExporterService(nc *v1alpha1.NebulaCluster) error {
	newSvc := e.generateService(nc)

	return syncService(newSvc, e.clientSet.Service())
}

func (e *nebulaExporter) syncExporterDeployment(nc *v1alpha1.NebulaCluster) error {
	newDeploy := e.generateDeployment(nc)

	oldDeploy, err := e.clientSet.Deployment().GetDeployment(newDeploy.Namespace, newDeploy.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	notExist := apierrors.IsNotFound(err)
	if notExist {
		if err := setDeploymentLastAppliedConfigAnnotation(newDeploy); err != nil {
			return err
		}
		return e.clientSet.Deployment().CreateDeployment(newDeploy)
	}

	return updateDeployment(e.clientSet, newDeploy, oldDeploy)
}

func (e *nebulaExporter) generateService(nc *v1alpha1.NebulaCluster) *corev1.Service {
	namespace := nc.GetNamespace()
	svcName := fmt.Sprintf("%s-exporter-svc", nc.GetName())
	labels := e.getExporterLabels(nc)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       defaultMetricsPort,
					TargetPort: intstr.FromInt(defaultMetricsPort),
				},
			},
		},
	}

	return service
}

func (e *nebulaExporter) generateDeployment(nc *v1alpha1.NebulaCluster) *appsv1.Deployment {
	namespace := nc.GetNamespace()
	ncName := nc.GetName()
	deployName := fmt.Sprintf("%s-exporter", nc.GetName())
	labels := e.getExporterLabels(nc)
	livenessProbe := nc.ExporterComponent().ComponentSpec().LivenessProbe()
	containers := make([]corev1.Container, 0)

	args := []string{
		"--listen-address=0.0.0.0:9100",
		fmt.Sprintf("--namespace=%s", namespace),
		fmt.Sprintf("--cluster=%s", ncName),
		fmt.Sprintf("--max-request=%d", nc.ExporterComponent().MaxRequests()),
	}
	if nc.ExporterComponent().CollectRegex() != "" {
		args = append(args, fmt.Sprintf("--collect=%s", nc.ExporterComponent().CollectRegex()))
	}
	if nc.ExporterComponent().IgnoreRegex() != "" {
		args = append(args, fmt.Sprintf("--ignore=%s", nc.ExporterComponent().IgnoreRegex()))
	}

	container := corev1.Container{
		Name:  "ng-exporter",
		Image: nc.ExporterComponent().ComponentSpec().PodImage(),
		Args:  args,
		Env:   nc.ExporterComponent().ComponentSpec().PodEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: defaultMetricsPort,
			},
		},
		VolumeMounts: nc.ExporterComponent().ComponentSpec().VolumeMounts(),
	}

	if livenessProbe != nil {
		container.ReadinessProbe = livenessProbe
	} else {
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Port:   intstr.FromInt(defaultMetricsPort),
					Path:   "/health",
					Scheme: corev1.URISchemeHTTP,
				},
			},
			FailureThreshold:    int32(2),
			InitialDelaySeconds: int32(30),
			TimeoutSeconds:      int32(5),
			PeriodSeconds:       int32(60),
		}
	}

	resources := nc.ExporterComponent().ComponentSpec().Resources()
	if resources != nil {
		container.Resources = *resources
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}

	containers = append(containers, container)
	containers = append(containers, nc.ExporterComponent().ComponentSpec().SidecarContainers()...)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployName,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(nc.ExporterComponent().ComponentSpec().Replicas()),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: nc.ExporterComponent().ComponentSpec().PodAnnotations(),
				},
				Spec: corev1.PodSpec{
					SchedulerName:      nc.Spec.SchedulerName,
					NodeSelector:       nc.ExporterComponent().ComponentSpec().NodeSelector(),
					InitContainers:     nc.ExporterComponent().ComponentSpec().InitContainers(),
					Containers:         containers,
					ImagePullSecrets:   nc.Spec.ImagePullSecrets,
					Affinity:           nc.ExporterComponent().ComponentSpec().Affinity(),
					Tolerations:        nc.ExporterComponent().ComponentSpec().Tolerations(),
					Volumes:            nc.ExporterComponent().ComponentSpec().Volumes(),
					ServiceAccountName: v1alpha1.NebulaServiceAccountName,
				},
			},
		},
	}

	return deployment
}

func (e *nebulaExporter) getExporterLabels(nc *v1alpha1.NebulaCluster) map[string]string {
	selector := label.New().Cluster(nc.GetName()).Exporter()
	labels := selector.Copy().Labels()
	podLabels := nc.Spec.Exporter.ComponentSpec.Labels

	return maputil.MergeStringMaps(true, labels, podLabels)
}

type FakeNebulaExporter struct {
	err error
}

func NewFakeNebulaExporter() *FakeNebulaExporter {
	return &FakeNebulaExporter{}
}

func (f *FakeNebulaExporter) SetReconcileError(err error) {
	f.err = err
}

func (f *FakeNebulaExporter) Reconcile(_ *v1alpha1.NebulaCluster) error {
	return f.err
}
