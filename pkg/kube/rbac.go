package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

func CheckRBAC(ctx context.Context, c client.Client, namespace string) error {
	if err := createServiceAccount(ctx, c, namespace); err != nil {
		return err
	}
	if err := createRole(ctx, c, namespace); err != nil {
		return err
	}
	if err := createRoleBinding(ctx, c, namespace); err != nil {
		return err
	}
	return nil
}

func createRole(ctx context.Context, k8sClient client.Client, namespace string) error {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.NebulaRoleName,
			Namespace: namespace,
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
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: v1alpha1.NebulaRoleName, Namespace: namespace}, &rbacv1.Role{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &role); err != nil {
				return fmt.Errorf("failed to create Role role: %v", err)
			}
		}
	}
	return nil
}

func createRoleBinding(ctx context.Context, k8sClient client.Client, namespace string) error {
	binding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.NebulaRoleBindingName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     v1alpha1.NebulaRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      v1alpha1.NebulaServiceAccountName,
				Namespace: namespace,
			},
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: v1alpha1.NebulaRoleBindingName, Namespace: namespace}, &rbacv1.RoleBinding{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &binding); err != nil {
				return fmt.Errorf("failed to create RoleBinding: %v", err)
			}
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
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: v1alpha1.NebulaServiceAccountName, Namespace: namespace}, &corev1.ServiceAccount{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &serviceAccount); err != nil {
				return fmt.Errorf("failed to create ServiceAccount: %v", err)
			}
		}
	}
	return nil
}
