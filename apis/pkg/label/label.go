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

package label

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// NodeHostnameLabelKey represents the name of node
	// This label is also used as part of the topology hierarchy
	NodeHostnameLabelKey = "kubernetes.io/hostname"
	// NodeRegionLabelKey represents a larger domain, made up of one or more zones
	NodeRegionLabelKey = "topology.kubernetes.io/region"
	// NodeZoneLabelKey represents a logical failure domain
	NodeZoneLabelKey = "topology.kubernetes.io/zone"
	// NameLabelKey represents the name of an application
	NameLabelKey string = "app.kubernetes.io/name"
	// ManagedByLabelKey represents the tool being used to manage the operation of an application
	ManagedByLabelKey string = "app.kubernetes.io/managed-by"
	// ComponentLabelKey represent the component within the architecture
	ComponentLabelKey string = "app.kubernetes.io/component"
	// ClusterLabelKey represents a unique name identifying the cluster of an application
	ClusterLabelKey string = "app.kubernetes.io/cluster"

	// NebulaOperator is ManagedByLabelKey label value
	NebulaOperator string = "nebula-operator"

	// GraphdLabelVal is graphd label value
	GraphdLabelVal string = "graphd"
	// MetadLabelVal is metad label value
	MetadLabelVal string = "metad"
	// StoragedLabelVal is storaged label value
	StoragedLabelVal string = "storaged"
	// ExporterLabelVal is exporter label value
	ExporterLabelVal string = "exporter"
	// ConsoleLabelVal is exporter label value
	ConsoleLabelVal string = "console"
)

type Label labels.Set

func New() Label {
	return map[string]string{
		NameLabelKey:      "nebula-graph",
		ManagedByLabelKey: NebulaOperator,
	}
}

func (l Label) Cluster(name string) Label {
	l[ClusterLabelKey] = name
	return l
}

func (l Label) Component(name string) Label {
	l[ComponentLabelKey] = name
	return l
}

func (l Label) Console() Label {
	return l.Component(ConsoleLabelVal)
}

func (l Label) Exporter() Label {
	return l.Component(ExporterLabelVal)
}

func (l Label) Graphd() Label {
	l.Component(GraphdLabelVal)
	return l
}

func (l Label) Metad() Label {
	l.Component(MetadLabelVal)
	return l
}

func (l Label) Storaged() Label {
	l.Component(StoragedLabelVal)
	return l
}

func (l Label) IsNebulaComponent() bool {
	componentLabelVal := l[ComponentLabelKey]
	return componentLabelVal == GraphdLabelVal ||
		componentLabelVal == MetadLabelVal ||
		componentLabelVal == StoragedLabelVal
}

func (l Label) Copy() Label {
	copyLabel := make(Label)
	for k, v := range l {
		copyLabel[k] = v
	}
	return copyLabel
}

func (l Label) Selector() (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(l.LabelSelector())
}

func (l Label) LabelSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: l}
}

func (l Label) IsManagedByNebulaOperator() bool {
	return l[ManagedByLabelKey] == NebulaOperator
}

func (l Label) IsGraphd() bool {
	return l[ComponentLabelKey] == GraphdLabelVal
}

func (l Label) IsMetad() bool {
	return l[ComponentLabelKey] == MetadLabelVal
}

func (l Label) IsStoraged() bool {
	return l[ComponentLabelKey] == StoragedLabelVal
}

func (l Label) Labels() Label {
	return l
}

func (l Label) String() string {
	return labels.Set(l).String()
}
