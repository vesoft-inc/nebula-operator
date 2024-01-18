//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	timex "time"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AgentContainerSpec) DeepCopyInto(out *AgentContainerSpec) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AgentContainerSpec.
func (in *AgentContainerSpec) DeepCopy() *AgentContainerSpec {
	if in == nil {
		return nil
	}
	out := new(AgentContainerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BRConfig) DeepCopyInto(out *BRConfig) {
	*out = *in
	if in.ClusterNamespace != nil {
		in, out := &in.ClusterNamespace, &out.ClusterNamespace
		*out = new(string)
		**out = **in
	}
	in.StorageProvider.DeepCopyInto(&out.StorageProvider)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BRConfig.
func (in *BRConfig) DeepCopy() *BRConfig {
	if in == nil {
		return nil
	}
	out := new(BRConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackupCondition) DeepCopyInto(out *BackupCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackupCondition.
func (in *BackupCondition) DeepCopy() *BackupCondition {
	if in == nil {
		return nil
	}
	out := new(BackupCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackupSpec) DeepCopyInto(out *BackupSpec) {
	*out = *in
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]v1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.BR != nil {
		in, out := &in.BR, &out.BR
		*out = new(BRConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.NumBackupsKeep != nil {
		in, out := &in.NumBackupsKeep, &out.NumBackupsKeep
		*out = new(int32)
		**out = **in
	}
	if in.ReservedTimeEpoch != nil {
		in, out := &in.ReservedTimeEpoch, &out.ReservedTimeEpoch
		*out = new(timex.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackupSpec.
func (in *BackupSpec) DeepCopy() *BackupSpec {
	if in == nil {
		return nil
	}
	out := new(BackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackupStatus) DeepCopyInto(out *BackupStatus) {
	*out = *in
	if in.TimeStarted != nil {
		in, out := &in.TimeStarted, &out.TimeStarted
		*out = (*in).DeepCopy()
	}
	if in.TimeCompleted != nil {
		in, out := &in.TimeCompleted, &out.TimeCompleted
		*out = (*in).DeepCopy()
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]BackupCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackupStatus.
func (in *BackupStatus) DeepCopy() *BackupStatus {
	if in == nil {
		return nil
	}
	out := new(BackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BalanceJob) DeepCopyInto(out *BalanceJob) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BalanceJob.
func (in *BalanceJob) DeepCopy() *BalanceJob {
	if in == nil {
		return nil
	}
	out := new(BalanceJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentSpec) DeepCopyInto(out *ComponentSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.EnvVars != nil {
		in, out := &in.EnvVars, &out.EnvVars
		*out = make([]v1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TopologySpreadConstraints != nil {
		in, out := &in.TopologySpreadConstraints, &out.TopologySpreadConstraints
		*out = make([]TopologySpreadConstraint, len(*in))
		copy(*out, *in)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(v1.SecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.InitContainers != nil {
		in, out := &in.InitContainers, &out.InitContainers
		*out = make([]v1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SidecarContainers != nil {
		in, out := &in.SidecarContainers, &out.SidecarContainers
		*out = make([]v1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]v1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VolumeMounts != nil {
		in, out := &in.VolumeMounts, &out.VolumeMounts
		*out = make([]v1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentSpec.
func (in *ComponentSpec) DeepCopy() *ComponentSpec {
	if in == nil {
		return nil
	}
	out := new(ComponentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentStatus) DeepCopyInto(out *ComponentStatus) {
	*out = *in
	if in.Workload != nil {
		in, out := &in.Workload, &out.Workload
		*out = new(WorkloadStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentStatus.
func (in *ComponentStatus) DeepCopy() *ComponentStatus {
	if in == nil {
		return nil
	}
	out := new(ComponentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConsoleSpec) DeepCopyInto(out *ConsoleSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConsoleSpec.
func (in *ConsoleSpec) DeepCopy() *ConsoleSpec {
	if in == nil {
		return nil
	}
	out := new(ConsoleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EmptyStruct) DeepCopyInto(out *EmptyStruct) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EmptyStruct.
func (in *EmptyStruct) DeepCopy() *EmptyStruct {
	if in == nil {
		return nil
	}
	out := new(EmptyStruct)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExporterSpec) DeepCopyInto(out *ExporterSpec) {
	*out = *in
	in.ComponentSpec.DeepCopyInto(&out.ComponentSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExporterSpec.
func (in *ExporterSpec) DeepCopy() *ExporterSpec {
	if in == nil {
		return nil
	}
	out := new(ExporterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GraphdServiceSpec) DeepCopyInto(out *GraphdServiceSpec) {
	*out = *in
	in.ServiceSpec.DeepCopyInto(&out.ServiceSpec)
	if in.LoadBalancerIP != nil {
		in, out := &in.LoadBalancerIP, &out.LoadBalancerIP
		*out = new(string)
		**out = **in
	}
	if in.ExternalTrafficPolicy != nil {
		in, out := &in.ExternalTrafficPolicy, &out.ExternalTrafficPolicy
		*out = new(v1.ServiceExternalTrafficPolicy)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GraphdServiceSpec.
func (in *GraphdServiceSpec) DeepCopy() *GraphdServiceSpec {
	if in == nil {
		return nil
	}
	out := new(GraphdServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GraphdSpec) DeepCopyInto(out *GraphdSpec) {
	*out = *in
	in.ComponentSpec.DeepCopyInto(&out.ComponentSpec)
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(GraphdServiceSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.LogVolumeClaim != nil {
		in, out := &in.LogVolumeClaim, &out.LogVolumeClaim
		*out = new(StorageClaim)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GraphdSpec.
func (in *GraphdSpec) DeepCopy() *GraphdSpec {
	if in == nil {
		return nil
	}
	out := new(GraphdSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LicenseSpec) DeepCopyInto(out *LicenseSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LicenseSpec.
func (in *LicenseSpec) DeepCopy() *LicenseSpec {
	if in == nil {
		return nil
	}
	out := new(LicenseSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogRotate) DeepCopyInto(out *LogRotate) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogRotate.
func (in *LogRotate) DeepCopy() *LogRotate {
	if in == nil {
		return nil
	}
	out := new(LogRotate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetadSpec) DeepCopyInto(out *MetadSpec) {
	*out = *in
	in.ComponentSpec.DeepCopyInto(&out.ComponentSpec)
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(ServiceSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.LogVolumeClaim != nil {
		in, out := &in.LogVolumeClaim, &out.LogVolumeClaim
		*out = new(StorageClaim)
		(*in).DeepCopyInto(*out)
	}
	if in.DataVolumeClaim != nil {
		in, out := &in.DataVolumeClaim, &out.DataVolumeClaim
		*out = new(StorageClaim)
		(*in).DeepCopyInto(*out)
	}
	if in.License != nil {
		in, out := &in.License, &out.License
		*out = new(LicenseSpec)
		**out = **in
	}
	if in.LicenseManagerURL != nil {
		in, out := &in.LicenseManagerURL, &out.LicenseManagerURL
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetadSpec.
func (in *MetadSpec) DeepCopy() *MetadSpec {
	if in == nil {
		return nil
	}
	out := new(MetadSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaBackup) DeepCopyInto(out *NebulaBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaBackup.
func (in *NebulaBackup) DeepCopy() *NebulaBackup {
	if in == nil {
		return nil
	}
	out := new(NebulaBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaBackupList) DeepCopyInto(out *NebulaBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NebulaBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaBackupList.
func (in *NebulaBackupList) DeepCopy() *NebulaBackupList {
	if in == nil {
		return nil
	}
	out := new(NebulaBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaCluster) DeepCopyInto(out *NebulaCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaCluster.
func (in *NebulaCluster) DeepCopy() *NebulaCluster {
	if in == nil {
		return nil
	}
	out := new(NebulaCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaClusterCondition) DeepCopyInto(out *NebulaClusterCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaClusterCondition.
func (in *NebulaClusterCondition) DeepCopy() *NebulaClusterCondition {
	if in == nil {
		return nil
	}
	out := new(NebulaClusterCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaClusterList) DeepCopyInto(out *NebulaClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NebulaCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaClusterList.
func (in *NebulaClusterList) DeepCopy() *NebulaClusterList {
	if in == nil {
		return nil
	}
	out := new(NebulaClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaClusterSpec) DeepCopyInto(out *NebulaClusterSpec) {
	*out = *in
	if in.Graphd != nil {
		in, out := &in.Graphd, &out.Graphd
		*out = new(GraphdSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Metad != nil {
		in, out := &in.Metad, &out.Metad
		*out = new(MetadSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Storaged != nil {
		in, out := &in.Storaged, &out.Storaged
		*out = new(StoragedSpec)
		(*in).DeepCopyInto(*out)
	}
	out.Reference = in.Reference
	if in.Suspend != nil {
		in, out := &in.Suspend, &out.Suspend
		*out = new(bool)
		**out = **in
	}
	if in.TopologySpreadConstraints != nil {
		in, out := &in.TopologySpreadConstraints, &out.TopologySpreadConstraints
		*out = make([]TopologySpreadConstraint, len(*in))
		copy(*out, *in)
	}
	if in.EnablePVReclaim != nil {
		in, out := &in.EnablePVReclaim, &out.EnablePVReclaim
		*out = new(bool)
		**out = **in
	}
	if in.ImagePullPolicy != nil {
		in, out := &in.ImagePullPolicy, &out.ImagePullPolicy
		*out = new(v1.PullPolicy)
		**out = **in
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]v1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.EnableBR != nil {
		in, out := &in.EnableBR, &out.EnableBR
		*out = new(bool)
		**out = **in
	}
	if in.EnableAutoFailover != nil {
		in, out := &in.EnableAutoFailover, &out.EnableAutoFailover
		*out = new(bool)
		**out = **in
	}
	out.FailoverPeriod = in.FailoverPeriod
	if in.LogRotate != nil {
		in, out := &in.LogRotate, &out.LogRotate
		*out = new(LogRotate)
		**out = **in
	}
	if in.Exporter != nil {
		in, out := &in.Exporter, &out.Exporter
		*out = new(ExporterSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Console != nil {
		in, out := &in.Console, &out.Console
		*out = new(ConsoleSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.SSLCerts != nil {
		in, out := &in.SSLCerts, &out.SSLCerts
		*out = new(SSLCertsSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Agent != nil {
		in, out := &in.Agent, &out.Agent
		*out = new(AgentContainerSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.AlpineImage != nil {
		in, out := &in.AlpineImage, &out.AlpineImage
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaClusterSpec.
func (in *NebulaClusterSpec) DeepCopy() *NebulaClusterSpec {
	if in == nil {
		return nil
	}
	out := new(NebulaClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaClusterStatus) DeepCopyInto(out *NebulaClusterStatus) {
	*out = *in
	in.Graphd.DeepCopyInto(&out.Graphd)
	in.Metad.DeepCopyInto(&out.Metad)
	in.Storaged.DeepCopyInto(&out.Storaged)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]NebulaClusterCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaClusterStatus.
func (in *NebulaClusterStatus) DeepCopy() *NebulaClusterStatus {
	if in == nil {
		return nil
	}
	out := new(NebulaClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaRestore) DeepCopyInto(out *NebulaRestore) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaRestore.
func (in *NebulaRestore) DeepCopy() *NebulaRestore {
	if in == nil {
		return nil
	}
	out := new(NebulaRestore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaRestore) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaRestoreList) DeepCopyInto(out *NebulaRestoreList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NebulaRestore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaRestoreList.
func (in *NebulaRestoreList) DeepCopy() *NebulaRestoreList {
	if in == nil {
		return nil
	}
	out := new(NebulaRestoreList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaRestoreList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaScheduledBackup) DeepCopyInto(out *NebulaScheduledBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaScheduledBackup.
func (in *NebulaScheduledBackup) DeepCopy() *NebulaScheduledBackup {
	if in == nil {
		return nil
	}
	out := new(NebulaScheduledBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaScheduledBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaScheduledBackupList) DeepCopyInto(out *NebulaScheduledBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NebulaScheduledBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaScheduledBackupList.
func (in *NebulaScheduledBackupList) DeepCopy() *NebulaScheduledBackupList {
	if in == nil {
		return nil
	}
	out := new(NebulaScheduledBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaScheduledBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RestoreCondition) DeepCopyInto(out *RestoreCondition) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RestoreCondition.
func (in *RestoreCondition) DeepCopy() *RestoreCondition {
	if in == nil {
		return nil
	}
	out := new(RestoreCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RestoreSpec) DeepCopyInto(out *RestoreSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.BR != nil {
		in, out := &in.BR, &out.BR
		*out = new(BRConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RestoreSpec.
func (in *RestoreSpec) DeepCopy() *RestoreSpec {
	if in == nil {
		return nil
	}
	out := new(RestoreSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RestoreStatus) DeepCopyInto(out *RestoreStatus) {
	*out = *in
	in.TimeStarted.DeepCopyInto(&out.TimeStarted)
	in.TimeCompleted.DeepCopyInto(&out.TimeCompleted)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]RestoreCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Partitions != nil {
		in, out := &in.Partitions, &out.Partitions
		*out = make(map[string][]*nebula.HostAddr, len(*in))
		for key, val := range *in {
			var outVal []*nebula.HostAddr
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]*nebula.HostAddr, len(*in))
				for i := range *in {
					if (*in)[i] != nil {
						in, out := &(*in)[i], &(*out)[i]
						*out = new(nebula.HostAddr)
						**out = **in
					}
				}
			}
			(*out)[key] = outVal
		}
	}
	if in.Checkpoints != nil {
		in, out := &in.Checkpoints, &out.Checkpoints
		*out = make(map[string]map[string]string, len(*in))
		for key, val := range *in {
			var outVal map[string]string
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make(map[string]string, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RestoreStatus.
func (in *RestoreStatus) DeepCopy() *RestoreStatus {
	if in == nil {
		return nil
	}
	out := new(RestoreStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *S3StorageProvider) DeepCopyInto(out *S3StorageProvider) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new S3StorageProvider.
func (in *S3StorageProvider) DeepCopy() *S3StorageProvider {
	if in == nil {
		return nil
	}
	out := new(S3StorageProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SSLCertsSpec) DeepCopyInto(out *SSLCertsSpec) {
	*out = *in
	if in.InsecureSkipVerify != nil {
		in, out := &in.InsecureSkipVerify, &out.InsecureSkipVerify
		*out = new(bool)
		**out = **in
	}
	if in.AutoMountServerCerts != nil {
		in, out := &in.AutoMountServerCerts, &out.AutoMountServerCerts
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SSLCertsSpec.
func (in *SSLCertsSpec) DeepCopy() *SSLCertsSpec {
	if in == nil {
		return nil
	}
	out := new(SSLCertsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScheduledBackupSpec) DeepCopyInto(out *ScheduledBackupSpec) {
	*out = *in
	if in.Pause != nil {
		in, out := &in.Pause, &out.Pause
		*out = new(bool)
		**out = **in
	}
	if in.MaxBackups != nil {
		in, out := &in.MaxBackups, &out.MaxBackups
		*out = new(int32)
		**out = **in
	}
	if in.MaxReservedTime != nil {
		in, out := &in.MaxReservedTime, &out.MaxReservedTime
		*out = new(string)
		**out = **in
	}
	in.BackupTemplate.DeepCopyInto(&out.BackupTemplate)
	if in.MaxSuccessfulNebulaBackupJobs != nil {
		in, out := &in.MaxSuccessfulNebulaBackupJobs, &out.MaxSuccessfulNebulaBackupJobs
		*out = new(int32)
		**out = **in
	}
	if in.MaxFailedNebulaBackupJobs != nil {
		in, out := &in.MaxFailedNebulaBackupJobs, &out.MaxFailedNebulaBackupJobs
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScheduledBackupSpec.
func (in *ScheduledBackupSpec) DeepCopy() *ScheduledBackupSpec {
	if in == nil {
		return nil
	}
	out := new(ScheduledBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScheduledBackupStatus) DeepCopyInto(out *ScheduledBackupStatus) {
	*out = *in
	if in.LastScheduledBackupTime != nil {
		in, out := &in.LastScheduledBackupTime, &out.LastScheduledBackupTime
		*out = (*in).DeepCopy()
	}
	if in.LastSuccessfulBackupTime != nil {
		in, out := &in.LastSuccessfulBackupTime, &out.LastSuccessfulBackupTime
		*out = (*in).DeepCopy()
	}
	if in.MostRecentJobFailed != nil {
		in, out := &in.MostRecentJobFailed, &out.MostRecentJobFailed
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScheduledBackupStatus.
func (in *ScheduledBackupStatus) DeepCopy() *ScheduledBackupStatus {
	if in == nil {
		return nil
	}
	out := new(ScheduledBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceSpec) DeepCopyInto(out *ServiceSpec) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ClusterIP != nil {
		in, out := &in.ClusterIP, &out.ClusterIP
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceSpec.
func (in *ServiceSpec) DeepCopy() *ServiceSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StorageClaim) DeepCopyInto(out *StorageClaim) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StorageClaim.
func (in *StorageClaim) DeepCopy() *StorageClaim {
	if in == nil {
		return nil
	}
	out := new(StorageClaim)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StorageProvider) DeepCopyInto(out *StorageProvider) {
	*out = *in
	if in.S3 != nil {
		in, out := &in.S3, &out.S3
		*out = new(S3StorageProvider)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StorageProvider.
func (in *StorageProvider) DeepCopy() *StorageProvider {
	if in == nil {
		return nil
	}
	out := new(StorageProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StoragedFailureHost) DeepCopyInto(out *StoragedFailureHost) {
	*out = *in
	if in.PVCSet != nil {
		in, out := &in.PVCSet, &out.PVCSet
		*out = make(map[types.UID]EmptyStruct, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.CreationTime.DeepCopyInto(&out.CreationTime)
	in.ConfirmationTime.DeepCopyInto(&out.ConfirmationTime)
	in.DeletionTime.DeepCopyInto(&out.DeletionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StoragedFailureHost.
func (in *StoragedFailureHost) DeepCopy() *StoragedFailureHost {
	if in == nil {
		return nil
	}
	out := new(StoragedFailureHost)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StoragedSpec) DeepCopyInto(out *StoragedSpec) {
	*out = *in
	in.ComponentSpec.DeepCopyInto(&out.ComponentSpec)
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(ServiceSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.LogVolumeClaim != nil {
		in, out := &in.LogVolumeClaim, &out.LogVolumeClaim
		*out = new(StorageClaim)
		(*in).DeepCopyInto(*out)
	}
	if in.DataVolumeClaims != nil {
		in, out := &in.DataVolumeClaims, &out.DataVolumeClaims
		*out = make([]StorageClaim, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.EnableAutoBalance != nil {
		in, out := &in.EnableAutoBalance, &out.EnableAutoBalance
		*out = new(bool)
		**out = **in
	}
	if in.EnableForceUpdate != nil {
		in, out := &in.EnableForceUpdate, &out.EnableForceUpdate
		*out = new(bool)
		**out = **in
	}
	if in.ConcurrentTransfer != nil {
		in, out := &in.ConcurrentTransfer, &out.ConcurrentTransfer
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StoragedSpec.
func (in *StoragedSpec) DeepCopy() *StoragedSpec {
	if in == nil {
		return nil
	}
	out := new(StoragedSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StoragedStatus) DeepCopyInto(out *StoragedStatus) {
	*out = *in
	in.ComponentStatus.DeepCopyInto(&out.ComponentStatus)
	if in.RemovedSpaces != nil {
		in, out := &in.RemovedSpaces, &out.RemovedSpaces
		*out = make([]int32, len(*in))
		copy(*out, *in)
	}
	if in.BalancedSpaces != nil {
		in, out := &in.BalancedSpaces, &out.BalancedSpaces
		*out = make([]int32, len(*in))
		copy(*out, *in)
	}
	if in.LastBalanceJob != nil {
		in, out := &in.LastBalanceJob, &out.LastBalanceJob
		*out = new(BalanceJob)
		**out = **in
	}
	if in.BalancedAfterFailover != nil {
		in, out := &in.BalancedAfterFailover, &out.BalancedAfterFailover
		*out = new(bool)
		**out = **in
	}
	if in.FailureHosts != nil {
		in, out := &in.FailureHosts, &out.FailureHosts
		*out = make(map[string]StoragedFailureHost, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StoragedStatus.
func (in *StoragedStatus) DeepCopy() *StoragedStatus {
	if in == nil {
		return nil
	}
	out := new(StoragedStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopologySpreadConstraint) DeepCopyInto(out *TopologySpreadConstraint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopologySpreadConstraint.
func (in *TopologySpreadConstraint) DeepCopy() *TopologySpreadConstraint {
	if in == nil {
		return nil
	}
	out := new(TopologySpreadConstraint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadReference) DeepCopyInto(out *WorkloadReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadReference.
func (in *WorkloadReference) DeepCopy() *WorkloadReference {
	if in == nil {
		return nil
	}
	out := new(WorkloadReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadStatus) DeepCopyInto(out *WorkloadStatus) {
	*out = *in
	if in.CollisionCount != nil {
		in, out := &in.CollisionCount, &out.CollisionCount
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadStatus.
func (in *WorkloadStatus) DeepCopy() *WorkloadStatus {
	if in == nil {
		return nil
	}
	out := new(WorkloadStatus)
	in.DeepCopyInto(out)
	return out
}
