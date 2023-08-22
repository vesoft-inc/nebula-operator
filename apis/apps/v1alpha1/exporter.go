/*
Copyright 2023 Vesoft Inc.

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

package v1alpha1

// +k8s:deepcopy-gen=false
type NebulaExporterComponent interface {
	ComponentSpec() ComponentAccessor
	MaxRequests() int32
	CollectRegex() string
	IgnoreRegex() string
}

var _ NebulaExporterComponent = &exporterComponent{}

// +k8s:deepcopy-gen=false
func newExporterComponent(nc *NebulaCluster) *exporterComponent {
	return &exporterComponent{nc: nc}
}

// +k8s:deepcopy-gen=false
type exporterComponent struct {
	nc *NebulaCluster
}

func (e *exporterComponent) ComponentSpec() ComponentAccessor {
	return buildComponentAccessor(e.nc, &e.nc.Spec.Exporter.ComponentSpec)
}

func (e *exporterComponent) MaxRequests() int32 {
	return e.nc.Spec.Exporter.MaxRequests
}

func (e *exporterComponent) CollectRegex() string {
	return e.nc.Spec.Exporter.CollectRegex
}

func (e *exporterComponent) IgnoreRegex() string {
	return e.nc.Spec.Exporter.IgnoreRegex
}
