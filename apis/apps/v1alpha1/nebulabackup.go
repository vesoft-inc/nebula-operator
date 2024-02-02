/*
Copyright 2024 Vesoft Inc.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (nb *NebulaBackup) GenerateOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion:         nb.APIVersion,
			Kind:               nb.Kind,
			Name:               nb.GetName(),
			UID:                nb.GetUID(),
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		},
	}
}

func (nb *NebulaBackup) CleanBackupData() bool {
	return pointer.BoolDeref(nb.Spec.CleanBackupData, false)
}

func (ncb *NebulaCronBackup) GenerateOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion:         ncb.APIVersion,
			Kind:               ncb.Kind,
			Name:               ncb.GetName(),
			UID:                ncb.GetUID(),
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		},
	}
}
