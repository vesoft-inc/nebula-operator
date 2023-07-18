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

package nebulacluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	storageutil "k8s.io/kubernetes/pkg/apis/storage/util"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	e2econfig "github.com/vesoft-inc/nebula-operator/tests/e2e/config"
	e2eutil "github.com/vesoft-inc/nebula-operator/tests/e2e/util"
)

func getNebulaCluster(runtimeClient client.Client, namespace, name string) *v1alpha1.NebulaCluster {
	imagePullPolicy := corev1.PullAlways
	storageClassName := getStorageClassName(runtimeClient)
	nebulaVersion := e2econfig.TestConfig.NebulaVersion
	return &v1alpha1.NebulaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.NebulaClusterSpec{
			Graphd: &v1alpha1.GraphdSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Image:    "vesoft/nebula-graphd",
					Version:  nebulaVersion,
				},
				LogVolumeClaim: &v1alpha1.StorageClaim{
					StorageClassName: &storageClassName,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
			Metad: &v1alpha1.MetadSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Image:    "vesoft/nebula-metad",
					Version:  nebulaVersion,
				},
				LogVolumeClaim: &v1alpha1.StorageClaim{
					StorageClassName: &storageClassName,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
				DataVolumeClaim: &v1alpha1.StorageClaim{
					StorageClassName: &storageClassName,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
			Storaged: &v1alpha1.StoragedSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Image:    "vesoft/nebula-storaged",
					Version:  nebulaVersion,
				},
				LogVolumeClaim: &v1alpha1.StorageClaim{
					StorageClassName: &storageClassName,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
				DataVolumeClaims: []v1alpha1.StorageClaim{
					{
						StorageClassName: &storageClassName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			Reference: v1alpha1.WorkloadReference{
				Name:    "statefulsets.apps",
				Version: "v1",
			},
			SchedulerName:   corev1.DefaultSchedulerName,
			ImagePullPolicy: &imagePullPolicy,
		},
	}
}

func getStorageClassName(runtimeClient client.Client) string {
	if e2econfig.TestConfig.StorageClass != "" {
		return e2econfig.TestConfig.StorageClass
	}

	var scList storagev1.StorageClassList
	err := runtimeClient.List(context.TODO(), &scList)
	framework.ExpectNoError(err, "failed to list StorageClass")
	framework.ExpectNotEqual(len(scList.Items), 0, "don't find StorageClass")
	var scName string
	for i := range scList.Items {
		sc := scList.Items[i]
		if storageutil.IsDefaultAnnotation(sc.ObjectMeta) {
			return sc.GetName()
		}
		if scName == "" {
			scName = sc.GetName()
		}
	}
	return scName
}

func waitForNebulaClusterReady(
	nc *v1alpha1.NebulaCluster,
	timeout, pollInterval time.Duration,
	runtimeClient client.Client,
) error {
	return wait.PollImmediate(pollInterval, timeout, func() (bool, error) {
		var err error
		var actual v1alpha1.NebulaCluster
		key := client.ObjectKey{Namespace: nc.Namespace, Name: nc.Name}
		err = runtimeClient.Get(context.TODO(), key, &actual)
		if err != nil {
			return false, nil
		}

		if !actual.MetadComponent().IsReady() {
			framework.Logf("Metad is not ready for NebulaCluster %s", key)
			return false, nil
		}
		if actual.Status.Metad.Phase != v1alpha1.RunningPhase {
			framework.Logf("Metad is in %s phase nor %s for NebulaCluster %s ",
				actual.Status.Metad.Phase, v1alpha1.RunningPhase, key)
			return false, nil
		}
		framework.Logf("Metad is ready and in %s phase for NebulaCluster %s", actual.Status.Metad.Phase, key)

		if !actual.StoragedComponent().IsReady() {
			framework.Logf("Storaged is not ready for NebulaCluster %s", key)
			return false, nil
		}
		if actual.Status.Storaged.Phase != v1alpha1.RunningPhase {
			framework.Logf("Storaged is in %s phase nor %s for NebulaCluster %s ",
				actual.Status.Storaged.Phase, v1alpha1.RunningPhase, key)
			return false, nil
		}
		framework.Logf("Storaged is ready and in %s phase for NebulaCluster %s", actual.Status.Storaged.Phase, key)

		if !actual.GraphdComponent().IsReady() {
			framework.Logf("Graphd is not ready for NebulaCluster %s", key)
			return false, nil
		}
		if actual.Status.Graphd.Phase != v1alpha1.RunningPhase {
			framework.Logf("Graphd is in %s phase nor %s for NebulaCluster %s ",
				actual.Status.Graphd.Phase, v1alpha1.RunningPhase, key)
			return false, nil
		}
		framework.Logf("Graphd is ready and in %s phase for NebulaCluster %s", actual.Status.Graphd.Phase, key)

		return true, nil
	})
}

func waitForNebulaClusterDeleted(
	nc *v1alpha1.NebulaCluster,
	timeout, pollInterval time.Duration,
	runtimeClient client.Client,
) error {
	key := client.ObjectKeyFromObject(nc)
	listOptions := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(label.New().Cluster(nc.GetClusterName()))),
		Namespace:     nc.Namespace,
	}

	checkIfDeleted := func(list client.ObjectList) (bool, error) {
		err := runtimeClient.List(context.TODO(), list, &listOptions)
		if err != nil {
			return false, err
		}
		switch v := list.(type) {
		case *appsv1.StatefulSetList:
			return len(v.Items) == 0, nil
		case *kruisev1alpha1.StatefulSetList:
			return len(v.Items) == 0, nil
		case *corev1.ServiceList:
			return len(v.Items) == 0, nil
		case *corev1.PersistentVolumeClaimList:
			return len(v.Items) == 0, nil
		}
		return false, fmt.Errorf("unkonw ObjectList %T", list)
	}

	objectLists := []client.ObjectList{
		&appsv1.StatefulSetList{},
		&kruisev1alpha1.StatefulSetList{},
		&corev1.ServiceList{},
	}

	if nc.IsPVReclaimEnabled() {
		objectLists = append(objectLists, &corev1.PersistentVolumeClaimList{})
	}

	return wait.PollImmediate(pollInterval, timeout, func() (bool, error) {
		var err error
		var actual v1alpha1.NebulaCluster
		framework.Logf("Waiting for NebulaCluster %s to be deleted", key)
		err = runtimeClient.Get(context.TODO(), key, &actual)
		if !apierrors.IsNotFound(err) {
			return false, nil
		}

		for _, list := range objectLists {
			if ok, _ := checkIfDeleted(list); !ok {
				framework.Logf("Waiting for NebulaCluster %s to be deleted, %T is not deleted", key, list)
				return false, nil
			}
		}

		return true, nil
	})
}

func updateNebulaCluster(
	nc *v1alpha1.NebulaCluster,
	runtimeClient client.Client,
	updateFunc func() error,
) error {
	key := client.ObjectKeyFromObject(nc)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := runtimeClient.Get(context.TODO(), key, nc); err != nil {
			return err
		}
		if err := updateFunc(); err != nil {
			return err
		}
		return runtimeClient.Update(context.TODO(), nc)
	})
}

type nebulaLog struct{}

func (l nebulaLog) Info(msg string)  { framework.Logf(msg) }
func (l nebulaLog) Warn(msg string)  { framework.Logf(msg) }
func (l nebulaLog) Error(msg string) { framework.Logf(msg) }
func (l nebulaLog) Fatal(msg string) { framework.Logf(msg) }

// nolint: unparam
func waitForExecuteNebulaSchema(
	timeout, pollInterval time.Duration,
	address string,
	port int,
	username, password, schema string,
) error {
	return wait.PollImmediate(pollInterval, timeout, func() (bool, error) {
		err := executeNebulaSchema(address, port, username, password, schema)
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

func executeNebulaSchema(address string, port int, username, password, schema string) error {
	hostAddress := nebulago.HostAddress{Host: address, Port: port}
	hostList := []nebulago.HostAddress{hostAddress}
	testPoolConfig := nebulago.GetDefaultConf()

	pool, err := nebulago.NewConnectionPool(hostList, testPoolConfig, nebulaLog{})
	if err != nil {
		framework.Logf("failed to initialize the connection pool, host: %s, port: %d, %s", address, port, err.Error())
		return err
	}
	defer pool.Close()

	session, err := pool.GetSession(username, password)
	if err != nil {
		framework.Logf("failed to create a new session from connection pool, username: %s, password: %s, %s",
			username, password, err.Error())
		return err
	}
	defer session.Release()

	resultSet, err := session.Execute(schema)
	if err != nil {
		framework.Logf("failed to execute schema %s, %s", schema, err.Error())
		return err
	}
	if !resultSet.IsSucceed() {
		framework.Logf("failed to execute schema %s, ErrorCode: %v, ErrorMsg: %s",
			schema, resultSet.GetErrorCode(), resultSet.GetErrorMsg())
		return fmt.Errorf("execute schema ErrorCode: %v, ErrorMsg: %s", resultSet.GetErrorCode(), resultSet.GetErrorMsg())
	}
	return nil
}

func portForwardNebulaClusterGraphd(
	nc *v1alpha1.NebulaCluster,
	localAddress string,
	config *rest.Config,
) (port int, stopChan chan<- struct{}, err error) {
	localPort, err := e2eutil.GetFreePort()
	if err != nil {
		return 0, nil, err
	}

	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})
	errCh := make(chan error)
	go func() {
		var out io.Writer
		err := (&e2eutil.PortForwardOptions{
			Namespace:    nc.Namespace,
			PodName:      nc.GraphdComponent().GetPodName(0),
			Config:       config,
			Address:      []string{localAddress},
			Ports:        []string{fmt.Sprintf("%d:%d", localPort, nc.GraphdComponent().GetPort(v1alpha1.GraphdPortNameThrift))},
			Out:          out,
			ErrOut:       os.Stderr,
			StopChannel:  stopCh,
			ReadyChannel: readyCh,
		}).RunPortForward()
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-readyCh:
		return localPort, stopCh, nil
	case err := <-errCh:
		return 0, nil, err
	}
}
