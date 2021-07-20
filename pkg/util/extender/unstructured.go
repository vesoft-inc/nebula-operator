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

package extender

import (
	"encoding/json"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/codec"
)

type UnstructuredExtender interface {
	GetSpec(obj *unstructured.Unstructured) map[string]interface{}
	GetTemplateMeta(obj *unstructured.Unstructured) map[string]interface{}
	GetTemplateSpec(obj *unstructured.Unstructured) map[string]interface{}
	GetStatus(obj *unstructured.Unstructured) map[string]interface{}
	GetReplicas(obj *unstructured.Unstructured) *int32
	GetContainers(obj *unstructured.Unstructured) []map[string]interface{}
	SetSpecField(obj *unstructured.Unstructured, value interface{}, fields ...string) error
	SetTemplateAnnotations(obj *unstructured.Unstructured, ann map[string]string) error
}

var _ UnstructuredExtender = &extender{}

type extender struct{}

func New() UnstructuredExtender {
	return &extender{}
}

func (e *extender) GetSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil
	}
	return spec
}

func (e *extender) GetTemplateMeta(obj *unstructured.Unstructured) map[string]interface{} {
	meta, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "metadata")
	if err != nil || !found {
		return nil
	}
	return meta
}

func (e *extender) GetTemplateSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "spec")
	if err != nil || !found {
		return nil
	}
	return spec
}

func (e *extender) GetStatus(obj *unstructured.Unstructured) map[string]interface{} {
	statusField, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return nil
	}
	return statusField
}

func (e *extender) GetReplicas(obj *unstructured.Unstructured) *int32 {
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil || !found {
		return nil
	}
	return pointer.Int32Ptr(int32(replicas))
}

func (e *extender) GetContainers(obj *unstructured.Unstructured) []map[string]interface{} {
	fields := []string{"spec", "template", "spec", "containers"}
	containers := make([]map[string]interface{}, 0)
	ctrs, ok, err := unstructured.NestedFieldNoCopy(obj.Object, fields...)
	if err != nil {
		return nil
	}
	if !ok {
		return nil
	}
	ctrList := ctrs.([]interface{})
	for _, container := range ctrList {
		ctr := container.(map[string]interface{})
		containers = append(containers, ctr)
	}
	return containers
}

func (e *extender) SetSpecField(obj *unstructured.Unstructured, value interface{}, fields ...string) error {
	return unstructured.SetNestedField(obj.Object, value, append([]string{"spec"}, fields...)...)
}

func (e *extender) SetTemplateAnnotations(obj *unstructured.Unstructured, ann map[string]string) error {
	fields := []string{"spec", "template", "metadata", "annotations"}
	oldAnn, _, err := unstructured.NestedStringMap(obj.Object, fields...)
	if err != nil {
		return err
	}

	if oldAnn == nil {
		oldAnn = make(map[string]string, len(ann))
	}

	for k, v := range ann {
		oldAnn[k] = v
	}

	return unstructured.SetNestedStringMap(obj.Object, oldAnn, fields...)
}

func IsUpdating(extender UnstructuredExtender, obj *unstructured.Unstructured) bool {
	status := extender.GetStatus(obj)
	if status == nil {
		return false
	}
	desiredReplicas := extender.GetReplicas(obj)

	if status["currentRevision"] == nil || status["updateRevision"] == nil || status["observedGeneration"] == nil {
		return false
	}

	if status["currentRevision"] != status["updateRevision"] {
		return true
	}

	if obj.GetGeneration() > status["observedGeneration"].(int64) && desiredReplicas == status["replicas"] {
		return true
	}
	return false
}

func templateEqual(oldTemplate, newTemplate map[string]interface{}) bool {
	var newVal, oldVal interface{}
	oldApply, _ := codec.Encode(oldTemplate)
	if err := json.Unmarshal([]byte(oldApply), &oldVal); err != nil {
		log.Error(err, "unmarshal failed")
		return false
	}
	newApply, _ := codec.Encode(newTemplate)
	if err := json.Unmarshal([]byte(newApply), &newVal); err != nil {
		log.Error(err, "unmarshal failed")
		return false
	}
	return apiequality.Semantic.DeepEqual(oldVal, newVal)
}

func PodTemplateEqual(extender UnstructuredExtender, newUnstruct, oldUnstruct *unstructured.Unstructured) bool {
	oldSpec := make(map[string]interface{})
	newPodTemplate := extender.GetTemplateSpec(newUnstruct)
	lastAppliedConfig, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastAppliedConfigKey]
	if ok {
		if err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec); err != nil {
			log.Error(err, "applied config failed",
				"namespace", oldUnstruct.GetNamespace(),
				"name", oldUnstruct.GetName())
			return false
		}
		oldPodTemplate, _, _ := unstructured.NestedMap(oldSpec, "template", "spec")
		return templateEqual(newPodTemplate, oldPodTemplate)
	}
	return false
}

func ObjectEqual(extender UnstructuredExtender, newUnstruct, oldUnstruct *unstructured.Unstructured) bool {
	annotations := map[string]string{}
	for k, v := range oldUnstruct.GetAnnotations() {
		if k != annotation.AnnLastAppliedConfigKey {
			annotations[k] = v
		}
	}
	if !apiequality.Semantic.DeepEqual(newUnstruct.GetAnnotations(), annotations) {
		return false
	}
	oldSpec := make(map[string]interface{})
	if lastAppliedConfig, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastAppliedConfigKey]; ok {
		if err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec); err != nil {
			log.Error(err, "unmarshal failed",
				"kind", oldUnstruct.GetKind(),
				"namespace", oldUnstruct.GetNamespace(),
				"name", oldUnstruct.GetName())
			return false
		}
		newSpec := extender.GetSpec(newUnstruct)
		return (int64(oldSpec["replicas"].(float64))) == newSpec["replicas"].(int64) &&
			templateEqual(oldSpec["template"].(map[string]interface{}), newSpec["template"].(map[string]interface{})) &&
			templateEqual(oldSpec["updateStrategy"].(map[string]interface{}), newSpec["updateStrategy"].(map[string]interface{}))
	}
	return false
}

func UpdateWorkload(
	workloadClient kube.Workload,
	extender UnstructuredExtender,
	newUnstruct,
	oldUnstruct *unstructured.Unstructured) error {
	isOrphan := metav1.GetControllerOf(oldUnstruct) == nil
	if !ObjectEqual(extender, newUnstruct, oldUnstruct) || isOrphan {
		w := oldUnstruct
		if err := extender.SetSpecField(w, extender.GetSpec(newUnstruct)["template"], "template"); err != nil {
			return err
		}
		annotations := annotation.CopyAnnotations(newUnstruct.GetAnnotations())
		v, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastSyncTimestampKey]
		if ok {
			annotations[annotation.AnnLastSyncTimestampKey] = v
		}
		w.SetAnnotations(annotations)
		var updateStrategy interface{}
		newSpec := extender.GetSpec(newUnstruct)
		if newSpec != nil {
			updateStrategy = newSpec["updateStrategy"]
			if err := extender.SetSpecField(w, updateStrategy, "updateStrategy"); err != nil {
				return err
			}
		}
		replicas := extender.GetReplicas(newUnstruct)
		if replicas != nil {
			if err := extender.SetSpecField(w, int64(*replicas), "replicas"); err != nil {
				return err
			}
		}
		if isOrphan {
			w.SetOwnerReferences(newUnstruct.GetOwnerReferences())
			w.SetLabels(newUnstruct.GetLabels())
		}
		if err := SetLastAppliedConfigAnnotation(extender, w); err != nil {
			return err
		}
		if err := workloadClient.UpdateWorkload(w); err != nil {
			return err
		}
	}
	return nil
}

func SetLastAppliedConfigAnnotation(extender UnstructuredExtender, obj *unstructured.Unstructured) error {
	spec := extender.GetSpec(obj)
	apply, err := codec.Encode(spec)
	if err != nil {
		return err
	}
	annotations := make(map[string]string)
	for k, v := range obj.GetAnnotations() {
		annotations[k] = v
	}
	annotations[annotation.AnnLastAppliedConfigKey] = apply
	obj.SetAnnotations(annotations)
	return nil
}

func SetUpdatePartition(
	extender UnstructuredExtender,
	obj *unstructured.Unstructured,
	upgradeOrdinal,
	gracePeriod int64,
	advanced bool) error {
	if err := extender.SetSpecField(obj, "RollingUpdate", "updateStrategy", "type"); err != nil {
		return err
	}
	if err := extender.SetSpecField(obj, upgradeOrdinal, "updateStrategy", "rollingUpdate", "partition"); err != nil {
		return err
	}
	if advanced {
		if err := extender.SetSpecField(obj, "InPlaceIfPossible", "updateStrategy", "rollingUpdate", "podUpdatePolicy"); err != nil {
			return err
		}
		if err := extender.SetSpecField(obj, gracePeriod,
			"updateStrategy", "rollingUpdate", "inPlaceUpdateStrategy", "gracePeriodSeconds"); err != nil {
			return err
		}
	}
	return nil
}

func SetContainerImage(obj *unstructured.Unstructured, containerName, image string) error {
	if image == "" {
		return nil
	}
	fields := []string{"spec", "template", "spec", "containers"}
	ctrs, ok, err := unstructured.NestedFieldCopy(obj.Object, fields...)
	if err != nil {
		return nil
	}
	if !ok {
		return nil
	}
	ctrList := ctrs.([]interface{})
	for _, container := range ctrList {
		ctr := container.(map[string]interface{})
		if ctr["name"] == containerName {
			ctr["image"] = image
		}
	}
	return unstructured.SetNestedField(obj.Object, ctrs, fields...)
}
