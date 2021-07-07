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

package console

import (
	"context"
	"testing"

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

func TestGenerateConsolePod(t *testing.T) {

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	fakeOptions := Options{
		Namespace:         "nebula",
		NebulaClusterName: "nc-test1",
		Image:             consoleDefaultImage,
		User:              "root",
		Password:          "*",
		runtimeCli:        fakeClient,
		restClientGetter:  nil,
	}

	fakeNebulaCluster := &v1alpha1.NebulaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeOptions.NebulaClusterName,
			Namespace: fakeOptions.Namespace,
		},
	}
	_ = fakeClient.Create(context.Background(), fakeNebulaCluster, &client.CreateOptions{})

	pod, err := fakeOptions.generateConsolePod()
	if err != nil {
		t.Error(err)
	}

	err = fakeOptions.runtimeCli.Create(context.Background(), pod, &client.CreateOptions{})
	if err != nil {
		t.Error(err)
	}

}
