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

package component

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	"github.com/vesoft-inc/nebula-operator/pkg/util/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/util/resource"
)

type metadUpdater struct {
	podClient kube.Pod
}

func NewMetadUpdater(podClient kube.Pod) UpdateManager {
	return &metadUpdater{podClient: podClient}
}

func (m *metadUpdater) Update(
	nc *v1alpha1.NebulaCluster,
	oldUnstruct, newUnstruct *unstructured.Unstructured,
	gvk schema.GroupVersionKind,
) error {
	if *nc.Spec.Metad.Replicas == int32(0) {
		return nil
	}

	if !extender.PodTemplateEqual(newUnstruct, oldUnstruct) {
		return nil
	}

	if nc.Status.Metad.Workload.UpdateRevision == nc.Status.Metad.Workload.CurrentRevision &&
		nc.Status.Metad.Phase == v1alpha1.RunningPhase {
		return nil
	}

	spec := extender.GetSpec(oldUnstruct)
	actualStrategy := spec["updateStrategy"].(map[string]interface{})
	partition := actualStrategy["rollingUpdate"].(map[string]interface{})
	advanced := gvk.GroupKind() == resource.AdvancedStatefulSetKind.GroupKind()
	if err := setPartition(newUnstruct, partition["partition"].(int64), advanced); err != nil {
		return err
	}
	replicas := extender.GetReplicas(oldUnstruct)
	index, err := getNextUpdatePod(nc.MetadComponent(), *replicas, m.podClient)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return utilerrors.ReconcileErrorf("%v", err)
		}
		return err
	}
	if index >= 0 {
		return m.updateMetadPod(index, newUnstruct, advanced)
	}

	return nil
}

func (m *metadUpdater) RestartPod(nc *v1alpha1.NebulaCluster, ordinal int32) error {
	return nil
}

func (m *metadUpdater) updateMetadPod(ordinal int32, newUnstruct *unstructured.Unstructured, advanced bool) error {
	return setPartition(newUnstruct, int64(ordinal), advanced)
}
