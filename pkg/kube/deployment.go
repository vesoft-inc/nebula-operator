package kube

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Deployment interface {
	CreateDeployment(deploy *appsv1.Deployment) error
	GetDeployment(namespace string, name string) (*appsv1.Deployment, error)
	UpdateDeployment(*appsv1.Deployment) error
}

type deployClient struct {
	kubecli client.Client
}

func NewDeployment(kubecli client.Client) Deployment {
	return &deployClient{kubecli: kubecli}
}

func (d *deployClient) CreateDeployment(deploy *appsv1.Deployment) error {
	if err := d.kubecli.Create(context.TODO(), deploy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("deployment [%s/%s] already exists", deploy.Namespace, deploy.Name)
			return nil
		}
		return err
	}
	return nil
}

func (d *deployClient) GetDeployment(namespace string, name string) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	err := d.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deploy)
	if err != nil {
		klog.Errorf("get deployment [%s/%s] failed: %v", namespace, name, err)
		return nil, err
	}
	return deploy, nil
}

func (d *deployClient) UpdateDeployment(deploy *appsv1.Deployment) error {
	ns := deploy.GetNamespace()
	deployName := deploy.GetName()
	spec := deploy.Spec.DeepCopy()
	labels := deploy.GetLabels()
	annotations := deploy.GetAnnotations()

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if updated, err := d.GetDeployment(ns, deployName); err == nil {
			deploy = updated.DeepCopy()
			deploy.Spec = *spec
			deploy.SetLabels(labels)
			deploy.SetAnnotations(annotations)
		} else {
			utilruntime.HandleError(fmt.Errorf("get deployment [%s/%s] failed: %v", ns, deployName, err))
			return err
		}

		updateErr := d.kubecli.Update(context.TODO(), deploy)
		if updateErr == nil {
			klog.Infof("deployment [%s/%s] updated successfully", ns, deployName)
			return nil
		}
		return updateErr
	})
}
