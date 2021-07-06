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

package check

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

func TestCheckNebulaCluster(t *testing.T) {

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	testcases := []struct {
		desc     string
		options  CheckOptions
		condType v1alpha1.NebulaClusterConditionType
		message  string
		expected string
	}{
		{
			desc: "",
			options: CheckOptions{
				Namespace:         "",
				NebulaClusterName: "nc-test1",
				runtimeCli:        fakeClient,
			},
			condType: v1alpha1.NebulaClusterReady,
			message:  "Nebula cluster is running",
			expected: "Nebula cluster is running",
		},
		{
			desc: "",
			options: CheckOptions{
				Namespace:         "nebula",
				NebulaClusterName: "nc-test2",
				runtimeCli:        fakeClient,
			},
			condType: v1alpha1.NebulaClusterReady,
			message:  "Nebula cluster is running",
			expected: "Nebula cluster is running",
		},
		{
			desc: "",
			options: CheckOptions{
				Namespace:         "nebula2",
				NebulaClusterName: "nc-test3",
				runtimeCli:        fakeClient,
			},
			expected: "",
		},
	}

	for _, tc := range testcases {
		fakeNebulaCluster := &v1alpha1.NebulaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tc.options.NebulaClusterName,
				Namespace: tc.options.Namespace,
			},
			Status: v1alpha1.NebulaClusterStatus{
				Conditions: []v1alpha1.NebulaClusterCondition{
					{
						Type:    tc.condType,
						Message: tc.message,
					},
				},
			},
		}
		_ = fakeClient.Create(context.Background(), fakeNebulaCluster, &client.CreateOptions{})
	}

	for i, tc := range testcases {
		str, err := tc.options.CheckNebulaCluster()
		if err != nil {
			t.Error(err)
		}
		if tc.expected != str {
			t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n", i, tc.expected, str)
		}
	}
}

func TestCheckPods(t *testing.T) {

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	testcases := []struct {
		desc       string
		options    CheckOptions
		phase      corev1.PodPhase
		conditions []corev1.PodCondition
		expected   string
	}{
		{
			options: CheckOptions{
				Namespace:         "",
				NebulaClusterName: "nc-test1",
				runtimeCli:        fakeClient,
			},
			phase: corev1.PodPending,
			conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Message: "0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.",
				},
			},
			expected: `Trouble Pods:
    PodName     Phase      ConditionType    Message
    -------     ------     -------------    -------
    nc-test1    Pending    PodScheduled     0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.
`,
		}, {
			options: CheckOptions{
				Namespace:         "nebula",
				NebulaClusterName: "nc-test2",
				runtimeCli:        fakeClient,
			},
			phase:      corev1.PodRunning,
			conditions: []corev1.PodCondition{},
			expected:   "All pods are running",
		}, {
			options: CheckOptions{
				Namespace:         "nebula2",
				NebulaClusterName: "nc-test3",
				runtimeCli:        fakeClient,
			},
			phase: corev1.PodFailed,
			conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Message: "0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.",
				},
				{
					Type:    corev1.PodInitialized,
					Status:  corev1.ConditionFalse,
					Message: "0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.",
				},
			},
			expected: `Trouble Pods:
    PodName     Phase     ConditionType    Message
    -------     ------    -------------    -------
    nc-test3    Failed    PodScheduled     0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.
    nc-test3    Failed    Initialized      0/5 nodes are available: 5 pod has unbound immediate PersistentVolumeClaims.
`,
		},
	}

	for _, tc := range testcases {
		fakePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tc.options.NebulaClusterName,
				Namespace: tc.options.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/cluster":    tc.options.NebulaClusterName,
					"app.kubernetes.io/managed-by": "nebula-operator",
					"app.kubernetes.io/name":       "nebula-graph",
				},
			},
			Status: corev1.PodStatus{
				Phase:      tc.phase,
				Conditions: tc.conditions,
			},
		}
		_ = fakeClient.Create(context.Background(), fakePod, &client.CreateOptions{})
	}

	for i, tc := range testcases {
		str, err := tc.options.CheckPods()
		if err != nil {
			t.Error(err)
		}
		if tc.expected != str {
			t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n", i, tc.expected, str)
		}
	}
}
