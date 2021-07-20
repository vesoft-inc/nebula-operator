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
