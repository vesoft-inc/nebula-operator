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

package provisioner

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
)

const (
	// AnnProvisionedBy is the external provisioner annotation in PV object
	AnnProvisionedBy = "pv.kubernetes.io/provisioned-by"

	// DefaultMountPoint is the mount point of raid device on the host under in knode
	DefaultMountPoint = "/mnt/disks/raid0"

	// DefaultFsType is the filesystem type to mount
	DefaultFsType = "ext4"

	// LocalFilesystemEnv will contain the filesystm path when script is invoked
	LocalFilesystemEnv = "LOCAL_PV_FILESYSTEM"
)

type CacheConfig struct {
	// Unique name of this provisioner
	ProvisionerName string
	// Cache to store PVs managed by this provisioner
	Cache *VolumeCache
	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory
}

type VolumeOptions struct {
	Name        string
	Path        string
	Node        string
	SizeInBytes int64
	Mode        corev1.PersistentVolumeMode
}
