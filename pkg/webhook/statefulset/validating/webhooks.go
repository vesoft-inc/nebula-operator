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

package validating

import (
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/apis/admission.nebula-graph.io/v1alpha1/statefulsetvalidating,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps,resources=statefulsets;statefulsets/scale,verbs=create;update,versions=v1,name=statefulsetvalidating.nebula-graph.io,admissionReviewVersions=v1

// +kubebuilder:webhook:path=/apis/admission.nebula-graph.io/v1alpha1/statefulsetvalidating,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kruise.io,resources=statefulsets;statefulsets/scale,verbs=create;update,versions=v1alpha1,name=kruisestatefulsetvalidating.nebula-graph.io,admissionReviewVersions=v1

// HandlerMap contains admission webhook handlers
var HandlerMap = map[string]admission.Handler{
	"/apis/admission.nebula-graph.io/v1alpha1/statefulsetvalidating": &StatefulSetCreateUpdateHandler{},
}
