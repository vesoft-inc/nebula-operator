package component

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
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
	labels := nc.GetExporterLabels()

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
	labels := nc.GetExporterLabels()
	livenessProbe := nc.GetExporterLivenessProbe()
	containers := make([]corev1.Container, 0)

	container := corev1.Container{
		Name:  "ng-exporter",
		Image: nc.GetExporterImage(),
		Args: []string{
			"--listen-address=0.0.0.0:9100", fmt.Sprintf("--namespace=%s", namespace),
			fmt.Sprintf("--cluster=%s", ncName), fmt.Sprintf("--max-request=%d", nc.Spec.Exporter.MaxRequests),
		},
		Env: nc.GetExporterEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: defaultMetricsPort,
			},
		},
	}

	if livenessProbe != nil {
		container.ReadinessProbe = livenessProbe
	} else {
		container.LivenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
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

	resources := nc.GetExporterResources()
	if resources != nil {
		container.Resources = *resources
	}

	imagePullPolicy := nc.Spec.ImagePullPolicy
	if imagePullPolicy != nil {
		container.ImagePullPolicy = *imagePullPolicy
	}

	containers = append(containers, container)
	containers = append(containers, nc.GetExporterSidecarContainers()...)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deployName,
			Namespace:       namespace,
			Labels:          labels,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: nc.GetExporterReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: nc.GetExporterPodAnnotations(),
				},
				Spec: corev1.PodSpec{
					SchedulerName:    nc.Spec.SchedulerName,
					NodeSelector:     nc.GetExporterNodeSelector(),
					InitContainers:   nc.GetExporterInitContainers(),
					Containers:       containers,
					ImagePullSecrets: nc.Spec.ImagePullSecrets,
					Affinity:         nc.GetExporterAffinity(),
					Tolerations:      nc.GetExporterTolerations(),
					Volumes:          nc.GetExporterSidecarVolumes(),
				},
			},
		},
	}

	return deployment
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
