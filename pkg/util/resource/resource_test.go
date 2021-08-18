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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/util/mock"
)

func TestGetGVKFromDefinition(t *testing.T) {
	dm := mock.NewMockDiscovery()
	dm.MockKindsFor = mock.NewMockKindsFor("NebulaGraph", "v1", "v2")
	gvk, err := GetGVKFromDefinition(dm, v1alpha1.WorkloadReference{Name: "testing.nebula.io"})
	assert.NoError(t, err)
	assert.Equal(t, schema.GroupVersionKind{
		Group:   "nebula.io",
		Version: "v1",
		Kind:    "NebulaGraph",
	}, gvk)

	gvk, err = GetGVKFromDefinition(dm, v1alpha1.WorkloadReference{Name: "testing.nebula.io", Version: "v2"})
	assert.NoError(t, err)
	assert.Equal(t, schema.GroupVersionKind{
		Group:   "nebula.io",
		Version: "v2",
		Kind:    "NebulaGraph",
	}, gvk)

	gvk, err = GetGVKFromDefinition(dm, v1alpha1.WorkloadReference{})
	assert.NoError(t, err)
	assert.Equal(t, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "StatefulSet",
	}, gvk)

	gvk, err = GetGVKFromDefinition(dm, v1alpha1.WorkloadReference{Name: "dummy"})
	assert.NoError(t, err)
	assert.Equal(t, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "NebulaGraph",
	}, gvk)
}
