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
	"time"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/label"
	e2eframework "github.com/vesoft-inc/nebula-operator/tests/e2e/framework"
)

var _ = ginkgo.Describe("NebulaCluster", func() {
	f := e2eframework.NewDefaultFramework("nebulacluster")

	var ns string
	var clientConfig *rest.Config
	var runtimeClient client.Client

	ginkgo.BeforeEach(func() {
		ns = f.Namespace.Name
		clientConfig = f.ClientConfig()
		runtimeClient = f.RuntimeClient
	})

	ginkgo.Describe("difference spec", func() {
		kruiseReference := v1alpha1.WorkloadReference{
			Name:    "statefulsets.apps.kruise.io",
			Version: "v1alpha1",
		}
		nebulaSchedulerName := "nebula-scheduler"

		testCases := []struct {
			Name             string
			GraphdReplicas   int32
			MetadReplicas    int32
			StoragedReplicas int32
			Reference        *v1alpha1.WorkloadReference
			SchedulerName    string
		}{
			{
				Name:             "test1-1-3",
				GraphdReplicas:   1,
				MetadReplicas:    1,
				StoragedReplicas: 3,
			}, {
				Name:             "test1-1-3",
				GraphdReplicas:   1,
				MetadReplicas:    1,
				StoragedReplicas: 3,
				Reference:        &kruiseReference,
				SchedulerName:    nebulaSchedulerName,
			}, {
				Name:             "test1-3-3",
				GraphdReplicas:   1,
				MetadReplicas:    3,
				StoragedReplicas: 3,
				SchedulerName:    nebulaSchedulerName,
			}, {
				Name:             "test1-3-3",
				GraphdReplicas:   1,
				MetadReplicas:    3,
				StoragedReplicas: 3,
				Reference:        &kruiseReference,
			}, {
				Name:             "test2-3-4",
				GraphdReplicas:   2,
				MetadReplicas:    3,
				StoragedReplicas: 4,
				SchedulerName:    nebulaSchedulerName,
			},
		}

		for i, tc := range testCases {
			tc := tc
			ginkgo.Context(fmt.Sprintf("%d: %#v", i, tc), func() {
				ginkgo.It("should deploy, scale out, and scale in successfully", func() {
					ginkgo.By("Deploy NebulaCluster")

					var err error
					// init the NebulaCluster Resource for testing
					nc := getNebulaCluster(ns, tc.Name)
					nc.Spec.Graphd.Replicas = pointer.Int32Ptr(tc.GraphdReplicas)
					nc.Spec.Metad.Replicas = pointer.Int32Ptr(tc.MetadReplicas)
					nc.Spec.Storaged.Replicas = pointer.Int32Ptr(tc.StoragedReplicas)
					if tc.Reference != nil {
						nc.Spec.Reference = *tc.Reference
					}
					if tc.SchedulerName != "" {
						nc.Spec.SchedulerName = tc.SchedulerName
					}

					err = runtimeClient.Create(context.TODO(), nc)
					framework.ExpectNoError(err, "failed to create NebulaCluster %s/%s", ns, nc.Name)

					ginkgo.By("Wait for NebulaCluster ready")

					err = waitForNebulaClusterReady(nc, 30*time.Minute, 30*time.Second, runtimeClient)
					framework.ExpectNoError(err, "failed to wait for NebulaCluster %s/%s ready", ns, nc.Name)

					ginkgo.By("Create port forward for NebulaCluster")

					graphLocalAddress := "127.0.0.1"
					var graphLocalPort int
					var stopCh chan<- struct{}
					graphLocalPort, stopCh, err = portForwardNebulaClusterGraphd(nc, graphLocalAddress, clientConfig)
					framework.ExpectNoError(err, "failed to port forward for graphd of NebulaCluster %s/%s", ns, nc.Name)
					defer close(stopCh)
					framework.Logf("create port forward %s:%d for graphd of NebulaCluster %s/%s", graphLocalAddress, graphLocalPort, ns, nc.Name)

					ginkgo.By("Init space and insert samples for NebulaCluster")
					replicaFactor := 3
					if tc.StoragedReplicas < 3 {
						replicaFactor = 1
					}
					executeSchema := fmt.Sprintf(
						"CREATE SPACE IF NOT EXISTS e2e_test(partition_num=%d,replica_factor=%d);",
						5*tc.StoragedReplicas, replicaFactor) +
						"USE e2e_test;" +
						"CREATE TAG IF NOT EXISTS person(name string, age int);" +
						"CREATE EDGE IF NOT EXISTS like(likeness double);"
					err = waitForExecuteNebulaSchema(30*time.Second, 2*time.Second, graphLocalAddress, graphLocalPort, "user", "pass", executeSchema)
					framework.ExpectNoError(err, "failed to init space after deploy for NebulaCluster %s/%s", ns, nc.Name)
					time.Sleep(10 * time.Second)

					executeSchema = "USE e2e_test;" +
						"INSERT VERTEX person(name, age) VALUES " +
						"'Bob':('Bob', 10), " +
						"'Lily':('Lily', 9), " +
						"'Tom':('Tom', 10), " +
						"'Jerry':('Jerry', 13), " +
						"'John':('John', 11);" +
						"INSERT EDGE like(likeness) VALUES " +
						"'Bob'->'Lily':(80.0);"
					err = waitForExecuteNebulaSchema(30*time.Second, 2*time.Second, graphLocalAddress, graphLocalPort, "user", "pass", executeSchema)
					framework.ExpectNoError(err, "failed to insert samples after deploy for NebulaCluster %s/%s", ns, nc.Name)

					ginkgo.By("Query from NebulaCluster")
					time.Sleep(2 * time.Second)
					executeSchema = "USE e2e_test;" +
						"GO FROM 'Bob' OVER like YIELD $^.person.name, $^.person.age, like.likeness;"
					err = waitForExecuteNebulaSchema(30*time.Second, 2*time.Second, graphLocalAddress, graphLocalPort, "user", "pass", executeSchema)
					framework.ExpectNoError(err, "failed to insert samples after scale out for NebulaCluster %s/%s", ns, nc.Name)

					ginkgo.By("Delete NebulaCluster")
					err = runtimeClient.Delete(context.TODO(), nc)
					framework.ExpectNoError(err, "failed to create NebulaCluster %s/%s", ns, nc.Name)

					ginkgo.By("Wait for NebulaCluster to be deleted")
					err = waitForNebulaClusterDeleted(nc, 10*time.Minute, 10*time.Second, runtimeClient)
					framework.ExpectNoError(err, "failed to wait for NebulaCluster %s/%s to be deleted", ns, nc.Name)
				})
			})
		}
	})

	ginkgo.It("Deleted resources controlled by NebulaCluster will be recovered", func() {
		ginkgo.By("Deploy NebulaCluster")
		var err error

		nc := getNebulaCluster(ns, "test-recovery")
		nc.Spec.Graphd.Replicas = pointer.Int32Ptr(2)
		nc.Spec.Metad.Replicas = pointer.Int32Ptr(3)
		nc.Spec.Storaged.Replicas = pointer.Int32Ptr(4)
		err = runtimeClient.Create(context.TODO(), nc)
		framework.ExpectNoError(err, "failed to create NebulaCluster %s/%s", ns, nc.Name)

		ginkgo.By("Wait for NebulaCluster ready")

		err = waitForNebulaClusterReady(nc, 30*time.Minute, 30*time.Second, runtimeClient)
		framework.ExpectNoError(err, "failed to wait for NebulaCluster %s/%s ready", ns, nc.Name)

		ginkgo.By("Delete StatefulSet/Service")
		listOptions := client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set(label.New().Cluster(nc.GetClusterName()))),
			Namespace:     ns,
		}
		stsList := appsv1.StatefulSetList{}
		err = runtimeClient.List(context.TODO(), &stsList, &listOptions)
		framework.ExpectNoError(err, "failed to list StatefulSet with option: %+v", listOptions)

		framework.Logf("delete StatefulSet %s/%d successfully", ns, len(stsList.Items))
		for i := range stsList.Items {
			sts := stsList.Items[i]
			err := runtimeClient.Delete(context.TODO(), &sts, &client.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete StatefulSet %s/%s", ns, sts.Name)
			framework.Logf("delete StatefulSet %s/%s successfully", ns, sts.Name)
		}

		svcList := corev1.ServiceList{}
		err = runtimeClient.List(context.TODO(), &svcList, &listOptions)
		framework.ExpectNoError(err, "failed to list Service with option: %+v", listOptions)
		for i := range svcList.Items {
			svc := svcList.Items[i]
			err := runtimeClient.Delete(context.TODO(), &svc, &client.DeleteOptions{})
			framework.ExpectNoError(err, "failed to delete Service %s/%s", ns, svc.Name)
		}

		ginkgo.By("Wait for NebulaCluster ready")

		err = waitForNebulaClusterReady(nc, 15*time.Minute, 10*time.Second, runtimeClient)
		framework.ExpectNoError(err, "failed to wait for NebulaCluster %s/%s ready", ns, nc.Name)
	})
})
