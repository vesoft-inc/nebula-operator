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
	"encoding/json"

	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/pkg/annotation"
)

type Ingress interface {
	CreateOrUpdateIngress(ingress *networkingv1.Ingress) error
	GetIngress(namespace, ingressName string) (*networkingv1.Ingress, error)
	DeleteIngress(namespace, ingressName string) error
}

type ingressClient struct {
	kubecli client.Client
}

func NewIngress(kubecli client.Client) Ingress {
	return &ingressClient{kubecli: kubecli}
}

func (i *ingressClient) CreateOrUpdateIngress(ingress *networkingv1.Ingress) error {
	log := getLog().WithValues("namespace", ingress.Namespace, "name", ingress.Name)
	if err := i.kubecli.Create(context.TODO(), ingress); err != nil {
		if apierrors.IsAlreadyExists(err) {
			merge := func(existing, desired *networkingv1.Ingress) error {
				if existing.Annotations == nil {
					existing.Annotations = map[string]string{}
				}
				for k, v := range desired.Annotations {
					existing.Annotations[k] = v
				}
				existing.Labels = desired.Labels
				equal, err := ingressEqual(desired, existing)
				if err != nil {
					return err
				}
				if !equal {
					b, err := json.Marshal(desired.Spec)
					if err != nil {
						return err
					}
					existing.Annotations[annotation.AnnLastAppliedConfigKey] = string(b)
					existing.Spec = desired.Spec
				}
				return nil
			}

			key := client.ObjectKeyFromObject(ingress)
			existing, err := i.getIngress(key)
			if err != nil {
				return err
			}

			mutated := existing.DeepCopy()
			if err := merge(mutated, ingress); err != nil {
				return err
			}

			if !apiequality.Semantic.DeepEqual(existing, mutated) {
				if err := i.updateIngress(mutated); err != nil {
					return err
				}
			}
		}
		return err
	}
	log.Info("ingress created")
	return nil
}

func (i *ingressClient) GetIngress(namespace, ingressName string) (*networkingv1.Ingress, error) {
	return i.getIngress(client.ObjectKey{Namespace: namespace, Name: ingressName})
}

func (i *ingressClient) getIngress(objKey client.ObjectKey) (*networkingv1.Ingress, error) {
	ingress := &networkingv1.Ingress{}
	err := i.kubecli.Get(context.TODO(), objKey, ingress)
	if err != nil {
		return nil, err
	}
	return ingress, err
}

func (i *ingressClient) updateIngress(ingress *networkingv1.Ingress) error {
	log := getLog().WithValues("namespace", ingress.Namespace, "name", ingress.Name)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return i.kubecli.Update(context.TODO(), ingress)
	})
	if err != nil {
		return err
	}
	log.Info("ingress updated")
	return nil
}

func (i *ingressClient) DeleteIngress(namespace, ingressName string) error {
	log := getLog().WithValues("namespace", namespace, "name", ingressName)
	ingress, err := i.getIngress(client.ObjectKey{Namespace: namespace, Name: ingressName})
	if err != nil {
		return err
	}
	log.Info("ingress deleted")
	return i.kubecli.Delete(context.TODO(), ingress)
}

func ingressEqual(newIngress, oldIngres *networkingv1.Ingress) (bool, error) {
	oldIngressSpec := networkingv1.IngressSpec{}
	if lastAppliedConfig, ok := oldIngres.Annotations[annotation.AnnLastAppliedConfigKey]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldIngressSpec)
		if err != nil {
			return false, err
		}
		return apiequality.Semantic.DeepEqual(oldIngressSpec, newIngress.Spec), nil
	}
	return false, nil
}
