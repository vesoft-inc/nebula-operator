/*
Copyright 2015 The Kubernetes Authors.
Copyright 2023 Vesoft Inc.

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

package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/vesoft-inc/nebula-operator/apis/autoscaling/v1alpha1"
)

var (
	// errSpec is used to determine if the error comes from the spec of HPA object in reconcileAutoscaler.
	// All such errors should have this error as a root error so that the upstream function can distinguish spec errors from internal errors.
	// e.g., fmt.Errorf("invalid spec%w", errSpec)
	errSpec = errors.New("")
)

// Computes the desired number of replicas for a specific hpa and metric specification,
// returning the metric status and a proposed condition to be set on the HPA object.
func (a *HorizontalController) computeReplicasForMetric(ctx context.Context, hpa *v1alpha1.NebulaAutoscaler, spec autoscalingv2.MetricSpec,
	specReplicas, statusReplicas int32, selector labels.Selector, status *autoscalingv2.MetricStatus) (replicaCountProposal int32, metricNameProposal string,
	timestampProposal time.Time, condition v1alpha1.NebulaAutoscalerCondition, err error) {

	switch spec.Type {
	case autoscalingv2.ObjectMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Object.Metric.Selector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForObjectMetric(specReplicas, statusReplicas, spec, hpa, selector, status, metricSelector)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
	case autoscalingv2.PodsMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Pods.Metric.Selector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForPodsMetric(specReplicas, spec, hpa, selector, status, metricSelector)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
	case autoscalingv2.ResourceMetricSourceType:
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForResourceMetric(ctx, specReplicas, spec, hpa, selector, status)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get %s resource metric value: %v", spec.Resource.Name, err)
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForContainerResourceMetric(ctx, specReplicas, spec, hpa, selector, status)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get %s container metric value: %v", spec.ContainerResource.Container, err)
		}
	case autoscalingv2.ExternalMetricSourceType:
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForExternalMetric(specReplicas, statusReplicas, spec, hpa, selector, status)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get %s external metric value: %v", spec.External.Metric.Name, err)
		}
	default:
		// It shouldn't reach here as invalid metric source type is filtered out in the api-server's validation.
		err = fmt.Errorf("unknown metric source type %q%w", string(spec.Type), errSpec)
		condition := a.getUnableComputeReplicaCountCondition(hpa, "InvalidMetricSourceType", err)
		return 0, "", time.Time{}, condition, err
	}
	return replicaCountProposal, metricNameProposal, timestampProposal, v1alpha1.NebulaAutoscalerCondition{}, nil
}

// computeStatusForObjectMetric computes the desired number of replicas for the specified metric of type ObjectMetricSourceType.
func (a *HorizontalController) computeStatusForObjectMetric(specReplicas, statusReplicas int32, metricSpec autoscalingv2.MetricSpec,
	hpa *v1alpha1.NebulaAutoscaler, selector labels.Selector, status *autoscalingv2.MetricStatus, metricSelector labels.Selector) (replicas int32,
	timestamp time.Time, metricName string, condition v1alpha1.NebulaAutoscalerCondition, err error) {
	if metricSpec.Object.Target.Type == autoscalingv2.ValueMetricType {
		replicaCountProposal, usageProposal, timestampProposal, err := a.replicaCalc.GetObjectMetricReplicas(specReplicas,
			metricSpec.Object.Target.Value.MilliValue(), metricSpec.Object.Metric.Name, hpa.Namespace, &metricSpec.Object.DescribedObject, selector, metricSelector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, timestampProposal, "", condition, err
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ObjectMetricSourceType,
			Object: &autoscalingv2.ObjectMetricStatus{
				DescribedObject: metricSpec.Object.DescribedObject,
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.Object.Metric.Name,
					Selector: metricSpec.Object.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					Value: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("%s metric %s", metricSpec.Object.DescribedObject.Kind, metricSpec.Object.Metric.Name), v1alpha1.NebulaAutoscalerCondition{}, nil
	} else if metricSpec.Object.Target.Type == autoscalingv2.AverageValueMetricType {
		replicaCountProposal, usageProposal, timestampProposal, err := a.replicaCalc.GetObjectPerPodMetricReplicas(statusReplicas,
			metricSpec.Object.Target.AverageValue.MilliValue(), metricSpec.Object.Metric.Name, hpa.Namespace, &metricSpec.Object.DescribedObject, metricSelector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, time.Time{}, "", condition, fmt.Errorf("failed to get %s object metric: %v", metricSpec.Object.Metric.Name, err)
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ObjectMetricSourceType,
			Object: &autoscalingv2.ObjectMetricStatus{
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.Object.Metric.Name,
					Selector: metricSpec.Object.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					AverageValue: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("external metric %s(%+v)", metricSpec.Object.Metric.Name, metricSpec.Object.Metric.Selector), v1alpha1.NebulaAutoscalerCondition{}, nil
	}
	errMsg := "invalid object metric source: neither a value target nor an average value target was set"
	err = fmt.Errorf(errMsg)
	condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
	return 0, time.Time{}, "", condition, err
}

// computeStatusForPodsMetric computes the desired number of replicas for the specified metric of type PodsMetricSourceType.
// computeStatusForResourceMetric computes the desired number of replicas for the specified metric of type ResourceMetricSourceType.
func (a *HorizontalController) computeStatusForResourceMetric(ctx context.Context, currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *v1alpha1.NebulaAutoscaler,
	selector labels.Selector, status *autoscalingv2.MetricStatus) (replicaCountProposal int32, timestampProposal time.Time,
	metricNameProposal string, condition v1alpha1.NebulaAutoscalerCondition, err error) {
	replicaCountProposal, metricValueStatus, timestampProposal, metricNameProposal, condition, err := a.computeStatusForResourceMetricGeneric(ctx,
		currentReplicas, metricSpec.Resource.Target, metricSpec.Resource.Name, hpa.Namespace, "", selector, autoscalingv2.ResourceMetricSourceType)
	if err != nil {
		condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetResourceMetric", err)
		return replicaCountProposal, timestampProposal, metricNameProposal, condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricStatus{
			Name:    metricSpec.Resource.Name,
			Current: *metricValueStatus,
		},
	}
	return replicaCountProposal, timestampProposal, metricNameProposal, condition, nil
}

// computeStatusForContainerResourceMetric computes the desired number of replicas for the specified metric of type ResourceMetricSourceType.
func (a *HorizontalController) computeStatusForContainerResourceMetric(ctx context.Context, currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *v1alpha1.NebulaAutoscaler,
	selector labels.Selector, status *autoscalingv2.MetricStatus) (replicaCountProposal int32, timestampProposal time.Time,
	metricNameProposal string, condition v1alpha1.NebulaAutoscalerCondition, err error) {
	replicaCountProposal, metricValueStatus, timestampProposal, metricNameProposal, condition, err := a.computeStatusForResourceMetricGeneric(ctx,
		currentReplicas, metricSpec.ContainerResource.Target, metricSpec.ContainerResource.Name, hpa.Namespace, metricSpec.ContainerResource.Container, selector, autoscalingv2.ContainerResourceMetricSourceType)
	if err != nil {
		condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetContainerResourceMetric", err)
		return replicaCountProposal, timestampProposal, metricNameProposal, condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.ContainerResourceMetricSourceType,
		ContainerResource: &autoscalingv2.ContainerResourceMetricStatus{
			Name:      metricSpec.ContainerResource.Name,
			Container: metricSpec.ContainerResource.Container,
			Current:   *metricValueStatus,
		},
	}
	return replicaCountProposal, timestampProposal, metricNameProposal, condition, nil
}

// computeStatusForExternalMetric computes the desired number of replicas for the specified metric of type ExternalMetricSourceType.
func (a *HorizontalController) computeStatusForExternalMetric(specReplicas, statusReplicas int32, metricSpec autoscalingv2.MetricSpec,
	hpa *v1alpha1.NebulaAutoscaler, selector labels.Selector, status *autoscalingv2.MetricStatus) (replicaCountProposal int32,
	timestampProposal time.Time, metricNameProposal string, condition v1alpha1.NebulaAutoscalerCondition, err error) {
	if metricSpec.External.Target.AverageValue != nil {
		replicaCountProposal, usageProposal, timestampProposal, err := a.replicaCalc.GetExternalPerPodMetricReplicas(statusReplicas,
			metricSpec.External.Target.AverageValue.MilliValue(), metricSpec.External.Metric.Name, hpa.Namespace, metricSpec.External.Metric.Selector)
		if err != nil {
			condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetExternalMetric", err)
			return 0, time.Time{}, "", condition, fmt.Errorf("failed to get %s external metric: %v", metricSpec.External.Metric.Name, err)
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ExternalMetricSourceType,
			External: &autoscalingv2.ExternalMetricStatus{
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.External.Metric.Name,
					Selector: metricSpec.External.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					AverageValue: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("external metric %s(%+v)", metricSpec.External.Metric.Name, metricSpec.External.Metric.Selector), v1alpha1.NebulaAutoscalerCondition{}, nil
	}
	if metricSpec.External.Target.Value != nil {
		replicaCountProposal, usageProposal, timestampProposal, err := a.replicaCalc.GetExternalMetricReplicas(specReplicas,
			metricSpec.External.Target.Value.MilliValue(), metricSpec.External.Metric.Name, hpa.Namespace, metricSpec.External.Metric.Selector, selector)
		if err != nil {
			condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetExternalMetric", err)
			return 0, time.Time{}, "", condition, fmt.Errorf("failed to get external metric %s: %v", metricSpec.External.Metric.Name, err)
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ExternalMetricSourceType,
			External: &autoscalingv2.ExternalMetricStatus{
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.External.Metric.Name,
					Selector: metricSpec.External.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					Value: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("external metric %s(%+v)", metricSpec.External.Metric.Name, metricSpec.External.Metric.Selector), v1alpha1.NebulaAutoscalerCondition{}, nil
	}
	errMsg := "invalid external metric source: neither a value target nor an average value target was set"
	err = fmt.Errorf(errMsg)
	condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetExternalMetric", err)
	return 0, time.Time{}, "", condition, fmt.Errorf(errMsg)
}

func (a *HorizontalController) computeStatusForPodsMetric(currentReplicas int32, metricSpec autoscalingv2.MetricSpec,
	hpa *v1alpha1.NebulaAutoscaler, selector labels.Selector, status *autoscalingv2.MetricStatus, metricSelector labels.Selector) (replicaCountProposal int32,
	timestampProposal time.Time, metricNameProposal string, condition v1alpha1.NebulaAutoscalerCondition, err error) {
	replicaCountProposal, usageProposal, timestampProposal, err := a.replicaCalc.GetMetricReplicas(currentReplicas,
		metricSpec.Pods.Target.AverageValue.MilliValue(), metricSpec.Pods.Metric.Name, hpa.Namespace, selector, metricSelector)
	if err != nil {
		condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
		return 0, timestampProposal, "", condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.PodsMetricSourceType,
		Pods: &autoscalingv2.PodsMetricStatus{
			Metric: autoscalingv2.MetricIdentifier{
				Name:     metricSpec.Pods.Metric.Name,
				Selector: metricSpec.Pods.Metric.Selector,
			},
			Current: autoscalingv2.MetricValueStatus{
				AverageValue: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
			},
		},
	}

	return replicaCountProposal, timestampProposal, fmt.Sprintf("pods metric %s", metricSpec.Pods.Metric.Name), v1alpha1.NebulaAutoscalerCondition{}, nil
}

func (a *HorizontalController) computeStatusForResourceMetricGeneric(ctx context.Context, currentReplicas int32, target autoscalingv2.MetricTarget,
	resourceName corev1.ResourceName, namespace string, container string, selector labels.Selector, sourceType autoscalingv2.MetricSourceType) (replicaCountProposal int32,
	metricStatus *autoscalingv2.MetricValueStatus, timestampProposal time.Time, metricNameProposal string,
	condition v1alpha1.NebulaAutoscalerCondition, err error) {
	if target.AverageValue != nil {
		var rawProposal int64
		replicaCountProposal, rawProposal, timestampProposal, err := a.replicaCalc.GetRawResourceReplicas(ctx,
			currentReplicas, target.AverageValue.MilliValue(), resourceName, namespace, selector, container)
		if err != nil {
			return 0, nil, time.Time{}, "", condition, fmt.Errorf("failed to get %s usage: %v", resourceName, err)
		}
		metricNameProposal = fmt.Sprintf("%s resource", resourceName.String())
		status := autoscalingv2.MetricValueStatus{
			AverageValue: resource.NewMilliQuantity(rawProposal, resource.DecimalSI),
		}
		return replicaCountProposal, &status, timestampProposal, metricNameProposal, v1alpha1.NebulaAutoscalerCondition{}, nil
	}

	if target.AverageUtilization == nil {
		errMsg := "invalid resource metric source: neither an average utilization target nor an average value (usage) target was set"
		return 0, nil, time.Time{}, "", condition, fmt.Errorf(errMsg)
	}

	targetUtilization := *target.AverageUtilization
	replicaCountProposal, percentageProposal, rawProposal, timestampProposal, err := a.replicaCalc.GetResourceReplicas(ctx, currentReplicas, targetUtilization, resourceName, namespace, selector, container)
	if err != nil {
		return 0, nil, time.Time{}, "", condition, fmt.Errorf("failed to get %s utilization: %v", resourceName, err)
	}

	metricNameProposal = fmt.Sprintf("%s resource utilization (percentage of request)", resourceName)
	if sourceType == autoscalingv2.ContainerResourceMetricSourceType {
		metricNameProposal = fmt.Sprintf("%s container resource utilization (percentage of request)", resourceName)
	}
	status := autoscalingv2.MetricValueStatus{
		AverageUtilization: &percentageProposal,
		AverageValue:       resource.NewMilliQuantity(rawProposal, resource.DecimalSI),
	}
	return replicaCountProposal, &status, timestampProposal, metricNameProposal, v1alpha1.NebulaAutoscalerCondition{}, nil
}
