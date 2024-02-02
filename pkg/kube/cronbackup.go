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

package kube

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

type NebulaCronBackup interface {
	GetCronBackup(namespace, name string) (*v1alpha1.NebulaCronBackup, error)
	UpdateCronBackupStatus(cronBackup *v1alpha1.NebulaCronBackup) error
}

type cronBackupClient struct {
	client client.Client
}

func NewCronNebulaBackup(client client.Client) NebulaCronBackup {
	return &cronBackupClient{client: client}
}

func (c *cronBackupClient) GetCronBackup(namespace, name string) (*v1alpha1.NebulaCronBackup, error) {
	cronBackup := &v1alpha1.NebulaCronBackup{}
	err := c.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cronBackup)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get NebulaCronBackup", "namespace", namespace, "name", name)
		return nil, err
	}
	return cronBackup, nil
}

func (c *cronBackupClient) UpdateCronBackupStatus(cronBackup *v1alpha1.NebulaCronBackup) error {
	ns := cronBackup.GetNamespace()
	cbName := cronBackup.GetName()
	status := cronBackup.Status.DeepCopy()

	return retry.OnError(retry.DefaultRetry, func(e error) bool { return e != nil }, func() error {
		cbClone, err := c.GetCronBackup(ns, cbName)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("get NebulaCronBackup [%s/%s] failed: %v", ns, cbName, err))
			return err
		}

		if reflect.DeepEqual(*status, cbClone.Status) {
			return nil
		}

		cronBackup = cbClone.DeepCopy()
		cronBackup.Status = *status
		updateErr := c.client.Status().Update(context.TODO(), cronBackup)
		if updateErr == nil {
			klog.Infof("NebulaCronBackup [%s/%s] status updated successfully", ns, cbName)
			return nil
		}
		klog.Errorf("update NebulaCronBackup [%s/%s] status failed: %v", ns, cbName, updateErr)
		return updateErr
	})
}
