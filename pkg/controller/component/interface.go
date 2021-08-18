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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type ReconcileManager interface {
	// Reconcile reconciles the cluster to desired state
	Reconcile(cluster *v1alpha1.NebulaCluster) error
}

type ScaleManager interface {
	// Scale scales the cluster
	Scale(nc *v1alpha1.NebulaCluster, old, new *unstructured.Unstructured) error
	// ScaleIn scales in the cluster
	ScaleIn(nc *v1alpha1.NebulaCluster, oldReplicas, newReplicas int32) error
	// ScaleOut scales out the cluster
	ScaleOut(nc *v1alpha1.NebulaCluster) error
}

type UpdateManager interface {
	// Update updates the cluster, as NebulaGraph doesn't support hot upgrade, the image tag remain unchanged
	Update(nc *v1alpha1.NebulaCluster, old, new *unstructured.Unstructured, gvk schema.GroupVersionKind) error
}
