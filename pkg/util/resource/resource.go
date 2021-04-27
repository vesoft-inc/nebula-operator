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

package resource

import (
	"encoding/json"
	"fmt"
	"reflect"

	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
)

type GVRFunc func() schema.GroupVersionResource

var (
	NebulaClusterKind       = v1alpha1.SchemeGroupVersion.WithKind("NebulaCluster")
	StatefulSetKind         = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
	AdvancedStatefulSetKind = kruisev1alpha1.SchemeGroupVersion.WithKind("StatefulSet")
	UnitedDeploymentKind    = kruisev1alpha1.SchemeGroupVersion.WithKind("UnitedDeployment")

	GroupVersionResources = map[string]GVRFunc{
		StatefulSetKind.String():         GetStatefulSetGVR,
		AdvancedStatefulSetKind.String(): GetAdvancedStatefulSetGVR,
		UnitedDeploymentKind.String():    GetUniteDeploymentGVR,
	}
)

func GetStatefulSetGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "statefulsets",
	}
}

func GetAdvancedStatefulSetGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps.kruise.io",
		Version:  "v1alpha1",
		Resource: "statefulsets",
	}
}

func GetUniteDeploymentGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps.kruise.io",
		Version:  "v1alpha1",
		Resource: "uniteddeployments",
	}
}

func GetGVKFromDefinition(dm discovery.Interface, ref v1alpha1.WorkloadReference) (schema.GroupVersionKind, error) {
	// if given definitionRef is empty return an default GVK
	if ref.Name == "" {
		return StatefulSetKind, nil
	}
	var gvk schema.GroupVersionKind
	groupResource := schema.ParseGroupResource(ref.Name)
	gvr := schema.GroupVersionResource{Group: groupResource.Group, Resource: groupResource.Resource, Version: ref.Version}
	kinds, err := dm.KindsFor(gvr)
	if err != nil {
		return gvk, err
	}
	if len(kinds) < 1 {
		return gvk, &meta.NoResourceMatchError{
			PartialResource: gvr,
		}
	}
	return kinds[0], nil
}

type Converters struct {
	Obj runtime.Object
}

func (c *Converters) ObjectToStatefulSet(set *appsv1.StatefulSet) error {
	return convertToTypedObj(c.Obj, set)
}

func (c *Converters) ObjectToAdvancedStatefulSet(set *kruisev1alpha1.StatefulSet) error {
	return convertToTypedObj(c.Obj, set)
}

func (c *Converters) ObjectToUnitedDeployment(deploy *kruisev1alpha1.UnitedDeployment) error {
	return convertToTypedObj(c.Obj, deploy)
}

func convertToTypedObj(obj runtime.Object, typedObj interface{}) error {
	un, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("expected \"*unstructured.Unstructured\", got \"%s\"", reflect.TypeOf(obj).Name())
	}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, typedObj)
	if err != nil {
		_ = fromUnstructuredViaJSON(un.Object, typedObj)
	}
	return nil
}

func fromUnstructuredViaJSON(u map[string]interface{}, obj interface{}) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func ConvertToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: objMap,
	}, nil
}
