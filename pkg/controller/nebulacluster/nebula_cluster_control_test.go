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
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component"
	"github.com/vesoft-inc/nebula-operator/pkg/controller/component/reclaimer"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

func Test_defaultNebulaClusterControl_UpdateNebulaCluster(t *testing.T) {
	g := NewGomegaWithT(t)

	type testcase struct {
		name                 string
		reconcileGraphdErr   bool
		reconcileMetadErr    bool
		reconcileStoragedErr bool
		metaReconcileErr     bool
		pvcReclaimErr        bool
		errExpectFn          func(*GomegaWithT, error)
	}

	testFn := func(test *testcase, t *testing.T) {
		t.Log(test.name)

		nc := newNebulaCluster()

		control, graphdCluster, metadCluster, storagedCluster, metaReconciler, pvcReclaimer := newFakeNebulaClusterControl()

		if test.reconcileGraphdErr {
			graphdCluster.SetReconcileError(fmt.Errorf("reconcile graphd cluster failed"))
		}
		if test.reconcileMetadErr {
			metadCluster.SetReconcileError(fmt.Errorf("reconcile metad cluster failed"))
		}
		if test.reconcileStoragedErr {
			storagedCluster.SetReconcileError(fmt.Errorf("reconcile storaged cluster failed"))
		}
		if test.metaReconcileErr {
			metaReconciler.SetReconcileError(fmt.Errorf("reconcile meta info failed"))
		}
		if test.pvcReclaimErr {
			pvcReclaimer.SetReclaimError(fmt.Errorf("reclaim pvc failed"))
		}

		err := control.UpdateNebulaCluster(nc)
		if test.errExpectFn != nil {
			test.errExpectFn(g, err)
		}
	}

	tests := []testcase{
		{
			name:               "reconcile graphd cluster failed",
			reconcileGraphdErr: true,
			errExpectFn: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.Contains(err.Error(), "reconcile graphd cluster failed")).To(Equal(true))
			},
		},
		{
			name:              "reconcile metad cluster failed",
			reconcileMetadErr: true,
			errExpectFn: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.Contains(err.Error(), "reconcile metad cluster failed")).To(Equal(true))
			},
		},
		{
			name:                 "reconcile storaged cluster failed",
			reconcileStoragedErr: true,
			errExpectFn: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.Contains(err.Error(), "reconcile storaged cluster failed")).To(Equal(true))
			},
		},
		{
			name:             "reconcile info failed",
			metaReconcileErr: true,
			errExpectFn: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.Contains(err.Error(), "reconcile meta info failed")).To(Equal(true))
			},
		},
		{
			name:          "reclaim pvc failed",
			pvcReclaimErr: true,
			errExpectFn: func(g *GomegaWithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(strings.Contains(err.Error(), "reclaim pvc failed")).To(Equal(true))
			},
		},
	}

	for i := range tests {
		testFn(&tests[i], t)
	}
}

func newFakeNebulaClusterControl() ( // nolint: gocritic
	ControlInterface,
	*component.FakeGraphdCluster,
	*component.FakeMetadCluster,
	*component.FakeStoragedCluster,
	*reclaimer.FakeMetaReconciler,
	*reclaimer.FakePVCReclaimer,
) {
	clientset := fake.NewClientBuilder().Build()
	fakeNebulaClient := kube.NewFakeNebulaCluster(clientset)
	graphdCluster := component.NewFakeGraphdCluster()
	metadCluster := component.NewFakeMetadCluster()
	storagedCluster := component.NewFakeStoragedCluster()
	exporter := component.NewFakeNebulaExporter()
	console := component.NewFakeNebulaConsole()
	metaReconciler := reclaimer.NewFakeMetaReconciler()
	pvcReclaimer := reclaimer.NewFakePVCReclaimer()
	control := NewDefaultNebulaClusterControl(
		clientset,
		fakeNebulaClient,
		graphdCluster,
		metadCluster,
		storagedCluster,
		exporter,
		console,
		metaReconciler,
		pvcReclaimer,
		&nebulaClusterConditionUpdater{})

	return control, graphdCluster, metadCluster, storagedCluster, metaReconciler, pvcReclaimer
}

func newNebulaCluster() *v1alpha1.NebulaCluster {
	return &v1alpha1.NebulaCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NebulaCluster",
			APIVersion: "apps.nebula-graph.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nebula",
			Namespace: corev1.NamespaceDefault,
			UID:       "123456",
		},
		Spec: v1alpha1.NebulaClusterSpec{
			Graphd: &v1alpha1.GraphdSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Image:   "vesoft/graphd",
					Version: "v3.5.0",
				},
			},
			Metad: &v1alpha1.MetadSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Image:   "vesoft/metad",
					Version: "v3.5.0",
				},
			},
			Storaged: &v1alpha1.StoragedSpec{
				ComponentSpec: v1alpha1.ComponentSpec{
					Replicas: pointer.Int32(1),
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Image:   "vesoft/storaged",
					Version: "v3.5.0",
				},
			},
		},
	}
}
