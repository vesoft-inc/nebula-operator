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

type CronJob interface {
	GetCronJob(namespace string, name string) (*batchv1.CronJob, error)
	CreateCronJob(job *batchv1.CronJob) error
	UpdateCronJob(job *batchv1.CronJob) error
	DeleteCronJob(namespace string, name string) error
}

type cronJobClient struct {
	kubecli client.Client
}

func NewCronJob(kubecli client.Client) CronJob {
	return &cronJobClient{kubecli: kubecli}
}

func (pd *cronJobClient) GetCronJob(namespace, name string) (*batchv1.CronJob, error) {
	cronjob := &batchv1.CronJob{}
	err := pd.kubecli.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, cronjob)
	if err != nil {
		klog.V(4).ErrorS(err, "failed to get cronjob", "namespace", namespace, "name", name)
		return nil, err
	}
	return cronjob, nil
}

func (pd *cronJobClient) CreateCronJob(job *batchv1.CronJob) error {
	if err := pd.kubecli.Create(context.TODO(), job); err != nil {
		if apierrors.IsAlreadyExists(err) && !strings.Contains(err.Error(), "being deleted") {
			return nil
		}
		return err
	}
	return nil
}

func (pd *cronJobClient) UpdateCronJob(job *batchv1.CronJob) error {
	return pd.kubecli.Update(context.TODO(), job, &client.UpdateOptions{})
}

func (pd *cronJobClient) DeleteCronJob(namespace, name string) error {
	cronjob := &batchv1.CronJob{}
	if err := pd.kubecli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, cronjob); err != nil {
		return err
	}

	policy := metav1.DeletePropagationBackground
	options := &client.DeleteOptions{
		PropagationPolicy: &policy,
	}
	return pd.kubecli.Delete(context.TODO(), cronjob, options)
}
