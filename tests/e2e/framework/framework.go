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

package framework

import (
	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Framework struct {
	*framework.Framework

	RuntimeClient client.Client
}

func NewDefaultFramework(baseName string) *Framework {
	bf := framework.NewDefaultFramework(baseName)
	f := &Framework{
		Framework: bf,
	}
	var runtimeClient client.Client

	ginkgo.BeforeEach(func() {
		var err error
		clientConfig := f.ClientConfig()

		runtimeClient, err = client.New(clientConfig, client.Options{})
		framework.ExpectNoError(err)
		f.RuntimeClient = runtimeClient
	})

	return f
}
