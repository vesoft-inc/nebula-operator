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

package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMap interface {
	CreateOrUpdateConfigMap(cm *corev1.ConfigMap) error
}

type cmClient struct {
	kubecli client.Client
}

func NewConfigMap(kubecli client.Client) ConfigMap {
	return &cmClient{kubecli: kubecli}
}

func (c *cmClient) CreateOrUpdateConfigMap(cm *corev1.ConfigMap) error {
	if err := c.kubecli.Create(context.TODO(), cm); err != nil {
		if apierrors.IsAlreadyExists(err) {
			merge := func(existing, desired *corev1.ConfigMap) error {
				existing.Data = desired.Data
				existing.Labels = desired.Labels
				for k, v := range desired.Annotations {
					existing.Annotations[k] = v
				}
				return nil
			}
			key := client.ObjectKeyFromObject(cm)
			existing, err := c.getConfigMap(key)
			if err != nil {
				return err
			}
			mutated := existing.DeepCopy()
			if err := merge(mutated, cm); err != nil {
				return err
			}
			if !apiequality.Semantic.DeepEqual(existing, mutated) {
				if err := c.updateConfigMap(mutated); err != nil {
					return err
				}
			}
		}
	}
	klog.V(1).Infof("namespace %s configMap %s created", cm.Namespace, cm.Name)
	return nil
}

func (c *cmClient) updateConfigMap(cm *corev1.ConfigMap) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := c.kubecli.Update(context.TODO(), cm); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	klog.Infof("namespace %s configMap %s updated", cm.Namespace, cm.Name)
	return nil
}

func (c *cmClient) getConfigMap(objKey client.ObjectKey) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := c.kubecli.Get(context.TODO(), objKey, configMap)
	if err != nil {
		return nil, err
	}
	return configMap, err
}
