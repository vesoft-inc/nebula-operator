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
	"strconv"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/util/codec"
)

func GetSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil
	}
	return spec
}

func GetTemplateSpec(obj *unstructured.Unstructured) map[string]interface{} {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "spec")
	if err != nil || !found {
		return nil
	}
	return spec
}

func GetStatus(obj *unstructured.Unstructured) map[string]interface{} {
	statusField, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return nil
	}
	return statusField
}

func GetReplicas(obj *unstructured.Unstructured) *int32 {
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil || !found {
		return nil
	}
	return pointer.Int32(int32(replicas))
}

func GetContainers(obj *unstructured.Unstructured) []map[string]interface{} {
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

func GetDataVolumeClaims(obj *unstructured.Unstructured) []map[string]interface{} {
	fields := []string{"spec", "volumeClaimTemplates"}
	volumeClaims := make([]map[string]interface{}, 0)
	vcs, ok, err := unstructured.NestedFieldCopy(obj.Object, fields...)
	if err != nil {
		return nil
	}
	if !ok {
		return nil
	}
	vcList := vcs.([]interface{})
	for _, volumeClaim := range vcList {
		vc := volumeClaim.(map[string]interface{})
		pvcObj := unstructured.Unstructured{
			Object: vc,
		}
		if strings.Contains(pvcObj.GetName(), "log") {
			continue
		}
		volumeClaims = append(volumeClaims, vc)
	}
	return volumeClaims
}

func SetSpecField(obj *unstructured.Unstructured, value interface{}, fields ...string) error {
	return unstructured.SetNestedField(obj.Object, value, append([]string{"spec"}, fields...)...)
}

func SetTemplateAnnotations(obj *unstructured.Unstructured, ann map[string]string) error {
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

func IsUpdating(obj *unstructured.Unstructured) bool {
	status := GetStatus(obj)
	if status == nil {
		return false
	}

	if status["currentRevision"] == nil || status["updateRevision"] == nil || status["observedGeneration"] == nil {
		return false
	}

	if status["currentRevision"] != status["updateRevision"] {
		return true
	}

	desiredReplicas := GetReplicas(obj)
	if obj.GetGeneration() > status["observedGeneration"].(int64) && int64(*desiredReplicas) == status["replicas"].(int64) {
		return true
	}
	return false
}

func templateEqual(oldTemplate, newTemplate map[string]interface{}) bool {
	var newVal, oldVal interface{}
	oldApply, _ := codec.Encode(oldTemplate)
	if err := json.Unmarshal([]byte(oldApply), &oldVal); err != nil {
		klog.Errorf("unmarshal failed, error: %v", err)
		return false
	}
	newApply, _ := codec.Encode(newTemplate)
	if err := json.Unmarshal([]byte(newApply), &newVal); err != nil {
		klog.Errorf("unmarshal failed, error: %v", err)
		return false
	}
	return apiequality.Semantic.DeepEqual(oldVal, newVal)
}

func PodTemplateEqual(newUnstruct, oldUnstruct *unstructured.Unstructured) bool {
	oldSpec := make(map[string]interface{})
	newPodTemplate := GetTemplateSpec(newUnstruct)
	lastAppliedConfig, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastAppliedConfigKey]
	if ok {
		if err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec); err != nil {
			klog.Errorf("applied config failed, error: %v", err)
			return false
		}
		oldPodTemplate, _, _ := unstructured.NestedMap(oldSpec, "template", "spec")
		return templateEqual(newPodTemplate, oldPodTemplate)
	}
	return false
}

func ObjectEqual(newUnstruct, oldUnstruct *unstructured.Unstructured) bool {
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
			klog.Errorf("unmarshal failed, error: %v", err)
			return false
		}
		newSpec := GetSpec(newUnstruct)
		return (int64(oldSpec["replicas"].(float64))) == newSpec["replicas"].(int64) &&
			templateEqual(oldSpec["template"].(map[string]interface{}), newSpec["template"].(map[string]interface{})) &&
			templateEqual(oldSpec["updateStrategy"].(map[string]interface{}), newSpec["updateStrategy"].(map[string]interface{}))
	}
	return false
}

func UpdateWorkload(
	workloadClient kube.Workload,
	newUnstruct,
	oldUnstruct *unstructured.Unstructured,
) error {
	isOrphan := metav1.GetControllerOf(oldUnstruct) == nil
	if !ObjectEqual(newUnstruct, oldUnstruct) || isOrphan {
		w := oldUnstruct
		if err := SetSpecField(w, GetSpec(newUnstruct)["template"], "template"); err != nil {
			return err
		}
		annotations := annotation.CopyAnnotations(newUnstruct.GetAnnotations())
		v, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastSyncTimestampKey]
		if ok {
			annotations[annotation.AnnLastSyncTimestampKey] = v
		}
		r, ok := oldUnstruct.GetAnnotations()[annotation.AnnLastReplicas]
		if ok {
			annotations[annotation.AnnLastReplicas] = r
		}
		t, ok := oldUnstruct.GetAnnotations()[annotation.AnnRestartTimestamp]
		if ok {
			annotations[annotation.AnnRestartTimestamp] = t
		}
		w.SetAnnotations(annotations)
		var updateStrategy interface{}
		newSpec := GetSpec(newUnstruct)
		if newSpec != nil {
			updateStrategy = newSpec["updateStrategy"]
			if err := SetSpecField(w, updateStrategy, "updateStrategy"); err != nil {
				return err
			}
		}
		replicas := GetReplicas(newUnstruct)
		if replicas != nil {
			if err := SetSpecField(w, int64(*replicas), "replicas"); err != nil {
				return err
			}
		}
		if isOrphan {
			w.SetOwnerReferences(newUnstruct.GetOwnerReferences())
			w.SetLabels(newUnstruct.GetLabels())
		}
		if err := SetLastAppliedConfigAnnotation(w); err != nil {
			return err
		}
		if err := workloadClient.UpdateWorkload(w); err != nil {
			return err
		}
	}
	return nil
}

func SetLastReplicasAnnotation(obj *unstructured.Unstructured) error {
	var lastReplicas int32
	val, ok := obj.GetAnnotations()[annotation.AnnLastReplicas]
	if ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		lastReplicas = int32(v)
	}
	replicas := pointer.Int32Deref(GetReplicas(obj), 0)
	if replicas == lastReplicas {
		return nil
	}
	annotations := make(map[string]string)
	for k, v := range obj.GetAnnotations() {
		annotations[k] = v
	}
	annotations[annotation.AnnLastReplicas] = strconv.Itoa(int(replicas))
	obj.SetAnnotations(annotations)
	return nil
}

func SetLastAppliedConfigAnnotation(obj *unstructured.Unstructured) error {
	spec := GetSpec(obj)
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

func SetRestartTimestamp(obj *unstructured.Unstructured) error {
	annotations := make(map[string]string)
	for k, v := range obj.GetAnnotations() {
		annotations[k] = v
	}
	annotations[annotation.AnnRestartTimestamp] = time.Now().Format(time.RFC3339)
	obj.SetAnnotations(annotations)
	return nil
}

func SetUpdatePartition(
	obj *unstructured.Unstructured,
	upgradeOrdinal,
	gracePeriod int64,
	advanced bool,
) error {
	if err := SetSpecField(obj, "RollingUpdate", "updateStrategy", "type"); err != nil {
		return err
	}
	if err := SetSpecField(obj, upgradeOrdinal, "updateStrategy", "rollingUpdate", "partition"); err != nil {
		return err
	}
	if advanced {
		if err := SetSpecField(obj, "InPlaceIfPossible", "updateStrategy", "rollingUpdate", "podUpdatePolicy"); err != nil {
			return err
		}
		if err := SetSpecField(obj, gracePeriod,
			"updateStrategy", "rollingUpdate", "inPlaceUpdateStrategy", "gracePeriodSeconds"); err != nil {
			return err
		}
	}
	return nil
}

func SetContainerImage(obj *unstructured.Unstructured, containerName, image string) error {
	fields := []string{"spec", "template", "spec", "containers"}
	ctrs, ok, err := unstructured.NestedFieldCopy(obj.Object, fields...)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	var match bool
	ctrList := ctrs.([]interface{})
	for _, container := range ctrList {
		ctr := container.(map[string]interface{})
		if ctr["name"] == containerName {
			ctr["image"] = image
			match = true
		}
	}
	if !match {
		return nil
	}
	return unstructured.SetNestedField(obj.Object, ctrs, fields...)
}
