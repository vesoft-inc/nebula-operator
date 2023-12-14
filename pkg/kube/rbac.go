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

package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

func CheckRBAC(ctx context.Context, c client.Client, namespace string) error {
	if err := createServiceAccount(ctx, c, namespace); err != nil {
		return err
	}
	if err := createClusterRole(ctx, c); err != nil {
		return err
	}
	if err := createClusterRoleBinding(ctx, c, namespace); err != nil {
		return err
	}
	return nil
}

func createClusterRole(ctx context.Context, k8sClient client.Client) error {
	role := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.NebulaRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts"},
				Verbs:     []string{"get", "list", "create", "update", "delete", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	if err := k8sClient.Create(ctx, &role); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create ClusterRole: %v", err)
	}
	return nil
}

func createClusterRoleBinding(ctx context.Context, k8sClient client.Client, namespace string) error {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.NebulaRoleBindingName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     v1alpha1.NebulaRoleName,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: v1alpha1.NebulaRoleBindingName}, binding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get ClusterRoleBinding: %v", err)
		}
		if err := k8sClient.Create(ctx, binding); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create ClusterRoleBinding: %v", err)
			}
		}
	}
	if !isApplied(binding.Subjects, namespace) {
		binding.Subjects = append(binding.Subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      v1alpha1.NebulaServiceAccountName,
			Namespace: namespace,
		})
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return k8sClient.Update(context.TODO(), binding)
		}); err != nil {
			return fmt.Errorf("failed to update ClusterRoleBinding subjects: %v", err)
		}
	}
	return nil
}

func createServiceAccount(ctx context.Context, k8sClient client.Client, namespace string) error {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.NebulaServiceAccountName,
			Namespace: namespace,
		},
	}
	if err := k8sClient.Create(ctx, &serviceAccount); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create nebula ServiceAccount: %v", err)
	}
	return nil
}

func isApplied(subjects []rbacv1.Subject, namespace string) bool {
	for _, sj := range subjects {
		if sj.Kind == "ServiceAccount" &&
			sj.Name == v1alpha1.NebulaServiceAccountName &&
			sj.Namespace == namespace {
			return true
		}
	}
	return false
}
