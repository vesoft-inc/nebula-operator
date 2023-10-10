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

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (nc *NebulaCluster) GraphdComponent() NebulaClusterComponent {
	return newGraphdComponent(nc)
}

func (nc *NebulaCluster) MetadComponent() NebulaClusterComponent {
	return newMetadComponent(nc)
}

func (nc *NebulaCluster) StoragedComponent() NebulaClusterComponent {
	return newStoragedComponent(nc)
}

func (nc *NebulaCluster) ExporterComponent() NebulaExporterComponent {
	return newExporterComponent(nc)
}

func (nc *NebulaCluster) ComponentByType(typ ComponentType) (NebulaClusterComponent, error) {
	switch typ {
	case GraphdComponentType:
		return nc.GraphdComponent(), nil
	case MetadComponentType:
		return nc.MetadComponent(), nil
	case StoragedComponentType:
		return nc.StoragedComponent(), nil
	}

	return nil, fmt.Errorf("unsupport component %s", typ)
}

func (nc *NebulaCluster) GetMetadThriftConnAddress() string {
	return nc.MetadComponent().GetConnAddress(MetadPortNameThrift)
}

func (nc *NebulaCluster) GetMetadEndpoints(portName string) []string {
	return nc.MetadComponent().GetEndpoints(portName)
}

func (nc *NebulaCluster) GetStoragedEndpoints(portName string) []string {
	return nc.StoragedComponent().GetEndpoints(portName)
}

func (nc *NebulaCluster) GetGraphdEndpoints(portName string) []string {
	return nc.GraphdComponent().GetEndpoints(portName)
}

func (nc *NebulaCluster) GetGraphdServiceName() string {
	return getServiceName(nc.GraphdComponent().GetName(), false)
}

func (nc *NebulaCluster) GetClusterName() string {
	return nc.Name
}

func (nc *NebulaCluster) GenerateOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion:         nc.APIVersion,
			Kind:               nc.Kind,
			Name:               nc.GetName(),
			UID:                nc.GetUID(),
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		},
	}
}

func (nc *NebulaCluster) IsPVReclaimEnabled() bool {
	return pointer.BoolDeref(nc.Spec.EnablePVReclaim, false)
}

func (nc *NebulaCluster) IsAutoBalanceEnabled() bool {
	return pointer.BoolDeref(nc.Spec.Storaged.EnableAutoBalance, false)
}

func (nc *NebulaCluster) IsForceUpdateEnabled() bool {
	return pointer.BoolDeref(nc.Spec.Storaged.EnableForceUpdate, false)
}

func (nc *NebulaCluster) ConcurrentTransfer() bool {
	return pointer.BoolDeref(nc.Spec.Storaged.ConcurrentTransfer, false)
}

func (nc *NebulaCluster) IsBREnabled() bool {
	return pointer.BoolDeref(nc.Spec.EnableBR, false)
}

func (nc *NebulaCluster) IsLogRotateEnabled() bool {
	return nc.Spec.LogRotate != nil
}

func (nc *NebulaCluster) InsecureSkipVerify() bool {
	return nc.Spec.SSLCerts != nil && pointer.BoolDeref(nc.Spec.SSLCerts.InsecureSkipVerify, false)
}

func (nc *NebulaCluster) AutoMountServerCerts() bool {
	return nc.Spec.SSLCerts != nil && pointer.BoolDeref(nc.Spec.SSLCerts.AutoMountServerCerts, false)
}

func (nc *NebulaCluster) IsIntraZoneReadingEnabled() bool {
	return nc.Spec.Graphd.Config["prioritize_intra_zone_reading"] == "true"
}

func (nc *NebulaCluster) IsGraphdSSLEnabled() bool {
	return nc.Spec.Graphd.Config["enable_graph_ssl"] == "true"
}

func (nc *NebulaCluster) IsMetadSSLEnabled() bool {
	return nc.Spec.Graphd.Config["enable_meta_ssl"] == "true" &&
		nc.Spec.Metad.Config["enable_meta_ssl"] == "true" &&
		nc.Spec.Storaged.Config["enable_meta_ssl"] == "true"
}

func (nc *NebulaCluster) IsClusterSSLEnabled() bool {
	return nc.Spec.Graphd.Config["enable_ssl"] == "true" &&
		nc.Spec.Metad.Config["enable_ssl"] == "true" &&
		nc.Spec.Storaged.Config["enable_ssl"] == "true"
}

func (nc *NebulaCluster) IsStoragedSSLEnabled() bool {
	return nc.Spec.Graphd.Config["enable_storage_ssl"] == "true" &&
		nc.Spec.Metad.Config["enable_storage_ssl"] == "true" &&
		nc.Spec.Storaged.Config["enable_storage_ssl"] == "true"
}

func (nc *NebulaCluster) IsZoneEnabled() bool {
	return nc.Spec.Metad.Config["zone_list"] != ""
}

func (nc *NebulaCluster) IsReady() bool {
	return nc.Status.ObservedGeneration == nc.Generation && nc.IsConditionReady()
}

func (nc *NebulaCluster) IsStoragedAvailable() bool {
	return nc.StoragedComponent().IsReady() &&
		nc.Status.Storaged.BalancedSpaces == nil &&
		nc.Status.Storaged.LastBalanceJob == nil
}

func (nc *NebulaCluster) IsConditionReady() bool {
	for _, condition := range nc.Status.Conditions {
		if condition.Type == NebulaClusterReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
