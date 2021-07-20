package mock

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"

	"github.com/vesoft-inc/nebula-operator/pkg/util/discovery"
)

var _ discovery.Interface = &Discovery{}

type GetServerVersion func() (*version.Info, error)
type GetMapper func() (meta.RESTMapper, error)
type Refresh func() (meta.RESTMapper, error)
type KindsFor func(input schema.GroupVersionResource) ([]schema.GroupVersionKind, error)

func NewMockDiscovery() *Discovery {
	return &Discovery{
		MockKindsFor: NewMockKindsFor(""),
	}
}

func NewMockKindsFor(kind string, version ...string) KindsFor {
	return func(input schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
		if kind == "" {
			return []schema.GroupVersionKind{{Version: input.Version, Group: input.Group, Kind: kind}}, nil
		}
		var ss []schema.GroupVersionKind
		for _, v := range version {
			gvk := schema.GroupVersionKind{Version: v, Group: input.Group, Kind: kind}
			if input.Version != "" && input.Version == v {
				return []schema.GroupVersionKind{gvk}, nil
			}
			ss = append(ss, gvk)
		}
		return ss, nil
	}
}

type Discovery struct {
	MockGetServerVersion GetServerVersion
	MockGetMapper        GetMapper
	MockRefresh          Refresh
	MockKindsFor         KindsFor
}

func (m *Discovery) GetServerVersion() (*version.Info, error) {
	return m.MockGetServerVersion()
}

func (m *Discovery) GetMapper() (meta.RESTMapper, error) {
	return m.MockGetMapper()
}

func (m *Discovery) Refresh() (meta.RESTMapper, error) {
	return m.MockRefresh()
}

func (m *Discovery) KindsFor(input schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return m.MockKindsFor(input)
}
