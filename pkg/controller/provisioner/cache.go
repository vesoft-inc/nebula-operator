/*
Copyright 2023 Vesoft Inc.
Copyright 2017 The Kubernetes Authors.

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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// VolumeCache keeps all the PersistentVolumes that have been created by this provisioner.
// It is periodically updated by the Populator.
// The Deleter and Discoverer use the VolumeCache to check on created PVs
type VolumeCache struct {
	mutex sync.Mutex
	pvs   map[string]*corev1.PersistentVolume
}

// NewVolumeCache creates a new PV cache object for storing PVs created by this provisioner.
func NewVolumeCache() *VolumeCache {
	return &VolumeCache{pvs: map[string]*corev1.PersistentVolume{}}
}

// GetPV returns the PV object given the PV name
func (cache *VolumeCache) GetPV(pvName string) (*corev1.PersistentVolume, bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	pv, exists := cache.pvs[pvName]
	return pv, exists
}

// AddPV adds the PV object to the cache
func (cache *VolumeCache) AddPV(pv *corev1.PersistentVolume) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.pvs[pv.Name] = pv
	klog.Infof("Added PV %s to cache", pv.Name)
}

// UpdatePV updates the PV object in the cache
func (cache *VolumeCache) UpdatePV(pv *corev1.PersistentVolume) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.pvs[pv.Name] = pv
	klog.V(4).Infof("Updated PV %s to cache", pv.Name)
}

// DeletePV deletes the PV object from the cache
func (cache *VolumeCache) DeletePV(pvName string) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	delete(cache.pvs, pvName)
	klog.Infof("Deleted PV %s from cache", pvName)
}

// ListPVs returns a list of all the PVs in the cache
func (cache *VolumeCache) ListPVs() []*corev1.PersistentVolume {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	var pvs []*corev1.PersistentVolume
	for _, pv := range cache.pvs {
		pvs = append(pvs, pv)
	}
	return pvs
}
