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
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Job interface {
	GetJob(namespace string, name string) (*batchv1.Job, error)
	CreateJob(job *batchv1.Job) error
	DeleteJob(namespace string, name string) error
}

type jobClient struct {
	client client.Client
}

func NewJob(client client.Client) Job {
	return &jobClient{client: client}
}

func (pd *jobClient) GetJob(namespace, name string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := pd.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, job)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get job", "namespace", namespace, "name", name)
		return nil, err
	}
	return job, nil
}

func (pd *jobClient) CreateJob(job *batchv1.Job) error {
	if err := pd.client.Create(context.TODO(), job); err != nil {
		if apierrors.IsAlreadyExists(err) && !strings.Contains(err.Error(), "being deleted") {
			return nil
		}
		return err
	}
	return nil
}

func (pd *jobClient) DeleteJob(namespace, name string) error {
	job := &batchv1.Job{}
	if err := pd.client.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, job); err != nil {
		return err
	}

	policy := metav1.DeletePropagationBackground
	options := &client.DeleteOptions{
		PropagationPolicy: &policy,
	}
	return pd.client.Delete(context.TODO(), job, options)
}
