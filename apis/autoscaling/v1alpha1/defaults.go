/*
Copyright 2023 Vesoft Inc.
Copyright 2021 The Kubernetes Authors.

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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

const DefaultCPUUtilization = 80

var (
	// These constants repeat previous HPA behavior
	scaleUpLimitPercent         int32 = 100
	scaleUpLimitMinimumPods     int32 = 4
	scaleUpPeriod               int32 = 15
	scaleUpStabilizationSeconds int32
	maxPolicy                   = autoscalingv2.MaxChangePolicySelect
	defaultHPAScaleUpRules      = autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: &scaleUpStabilizationSeconds,
		SelectPolicy:               &maxPolicy,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         scaleUpLimitMinimumPods,
				PeriodSeconds: scaleUpPeriod,
			},
			{
				Type:          autoscalingv2.PercentScalingPolicy,
				Value:         scaleUpLimitPercent,
				PeriodSeconds: scaleUpPeriod,
			},
		},
	}
	scaleDownPeriod               int32 = 15
	scaleDownStabilizationSeconds int32 = 300
	// Currently we can set the downscaleStabilizationWindow from the command line
	// So we can not rewrite the command line option from here
	scaleDownLimitPercent    int32 = 100
	defaultHPAScaleDownRules       = autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: &scaleDownStabilizationSeconds,
		SelectPolicy:               &maxPolicy,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PercentScalingPolicy,
				Value:         scaleDownLimitPercent,
				PeriodSeconds: scaleDownPeriod,
			},
		},
	}
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_NebulaAutoscaler(obj *NebulaAutoscaler) {
	if obj.Spec.GraphdPolicy.MinReplicas == nil {
		obj.Spec.GraphdPolicy.MinReplicas = pointer.Int32(1)
	}

	if len(obj.Spec.GraphdPolicy.Metrics) == 0 {
		utilizationDefaultVal := int32(DefaultCPUUtilization)
		obj.Spec.GraphdPolicy.Metrics = []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &utilizationDefaultVal,
					},
				},
			},
		}
	}
	SetDefaults_NebulaAutoscalerBehavior(obj)
}

// SetDefaults_NebulaAutoscalerBehavior fills the behavior if it is not null
func SetDefaults_NebulaAutoscalerBehavior(obj *NebulaAutoscaler) {
	// if behavior is specified, we should fill all the 'nil' values with the default ones
	if obj.Spec.GraphdPolicy.Behavior != nil {
		obj.Spec.GraphdPolicy.Behavior.ScaleUp = GenerateHPAScaleUpRules(obj.Spec.GraphdPolicy.Behavior.ScaleUp)
		obj.Spec.GraphdPolicy.Behavior.ScaleDown = GenerateHPAScaleDownRules(obj.Spec.GraphdPolicy.Behavior.ScaleDown)
	}
}

// GenerateHPAScaleUpRules returns a fully-initialized HPAScalingRules value
// We guarantee that no pointer in the structure will have the 'nil' value
func GenerateHPAScaleUpRules(scalingRules *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	defaultScalingRules := defaultHPAScaleUpRules.DeepCopy()
	return copyHPAScalingRules(scalingRules, defaultScalingRules)
}

// GenerateHPAScaleDownRules returns a fully-initialized HPAScalingRules value
// We guarantee that no pointer in the structure will have the 'nil' value
// EXCEPT StabilizationWindowSeconds, for reasoning check the comment for defaultHPAScaleDownRules
func GenerateHPAScaleDownRules(scalingRules *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	defaultScalingRules := defaultHPAScaleDownRules.DeepCopy()
	return copyHPAScalingRules(scalingRules, defaultScalingRules)
}

// copyHPAScalingRules copies all non-`nil` fields in HPA constraint structure
func copyHPAScalingRules(from, to *autoscalingv2.HPAScalingRules) *autoscalingv2.HPAScalingRules {
	if from == nil {
		return to
	}
	if from.SelectPolicy != nil {
		to.SelectPolicy = from.SelectPolicy
	}
	if from.StabilizationWindowSeconds != nil {
		to.StabilizationWindowSeconds = from.StabilizationWindowSeconds
	}
	if from.Policies != nil {
		to.Policies = from.Policies
	}
	return to
}
