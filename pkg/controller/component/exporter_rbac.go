package component

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

const (
	exporterRoleName           = "exporter-role"
	exporterRoleBindingName    = "exporter-rolebinding"
	exporterServiceAccountName = "exporter-sa"
)

func (e *nebulaExporter) checkExporterRBAC(nc *v1alpha1.NebulaCluster) error {
	if err := e.createServiceAccount(nc); err != nil {
		return err
	}
	if err := e.createRole(nc); err != nil {
		return err
	}
	if err := e.createRoleBinding(nc); err != nil {
		return err
	}
	return nil
}

func (e *nebulaExporter) createServiceAccount(nc *v1alpha1.NebulaCluster) error {
	var serviceAccount = v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            exporterServiceAccountName,
			Namespace:       nc.Namespace,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
	}
	if _, err := e.clientSet.ServiceAccount().GetServiceAccount(nc.Namespace, exporterServiceAccountName); err != nil {
		if apierrors.IsNotFound(err) {
			if err = e.clientSet.ServiceAccount().CreateServiceAccount(&serviceAccount); err != nil {
				return fmt.Errorf("failed to create ServiceAccount: %v", err)
			}
		}
	}
	return nil
}

func (e *nebulaExporter) createRole(nc *v1alpha1.NebulaCluster) error {
	var role = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            exporterRoleName,
			Namespace:       nc.Namespace,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			},
		},
	}
	if _, err := e.clientSet.Role().GetRole(nc.Namespace, exporterRoleName); err != nil {
		if apierrors.IsNotFound(err) {
			if err = e.clientSet.Role().CreateRole(&role); err != nil {
				return fmt.Errorf("failed to create Role: %v", err)
			}
		}
	}
	return nil
}

func (e *nebulaExporter) createRoleBinding(nc *v1alpha1.NebulaCluster) error {
	var roleBinding = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            exporterRoleBindingName,
			Namespace:       nc.Namespace,
			OwnerReferences: nc.GenerateOwnerReferences(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     exporterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      exporterServiceAccountName,
				Namespace: nc.Namespace,
			},
		},
	}

	if _, err := e.clientSet.RoleBinding().GetRoleBinding(nc.Namespace, exporterRoleBindingName); err != nil {
		if apierrors.IsNotFound(err) {
			if err = e.clientSet.RoleBinding().CreateRoleBinding(&roleBinding); err != nil {
				return fmt.Errorf("failed to create RoleBinding: %v", err)
			}
		}
	}

	return nil
}
