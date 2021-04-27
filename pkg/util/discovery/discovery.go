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

package discovery

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type Interface interface {
	GetMapper() (meta.RESTMapper, error)
	GetServerVersion() (*version.Info, error)
	Refresh() (meta.RESTMapper, error)
	KindsFor(input schema.GroupVersionResource) ([]schema.GroupVersionKind, error)
}

var _ Interface = &Discovery{}

type Discovery struct {
	dc     *discovery.DiscoveryClient
	mapper meta.RESTMapper
}

func New(c *rest.Config) (Interface, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		return nil, err
	}
	return &Discovery{
		dc: dc,
	}, nil
}

func (d *Discovery) GetMapper() (meta.RESTMapper, error) {
	if d.mapper == nil {
		return d.Refresh()
	}
	return d.mapper, nil
}

func (d *Discovery) GetServerVersion() (*version.Info, error) {
	ver, err := d.dc.ServerVersion()
	if err != nil {
		return nil, err
	}
	return ver, nil
}

func (d *Discovery) Refresh() (meta.RESTMapper, error) {
	gr, err := restmapper.GetAPIGroupResources(d.dc)
	if err != nil {
		return nil, err
	}
	d.mapper = restmapper.NewDiscoveryRESTMapper(gr)
	return d.mapper, nil
}

func (d *Discovery) KindsFor(input schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	mapper, err := d.GetMapper()
	if err != nil {
		return nil, err
	}
	mapping, err := mapper.KindsFor(input)
	if meta.IsNoMatchError(err) {
		// if no kind match err, refresh and try once more.
		mapper, err = d.Refresh()
		if err != nil {
			return nil, err
		}
		mapping, err = mapper.KindsFor(input)
	}
	return mapping, err
}
