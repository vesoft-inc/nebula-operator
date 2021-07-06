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

package util

import (
	"sync"
	"testing"

	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/config"
)

func TestGetNebulaClusterConfig(t *testing.T) {
	testcases := []struct {
		desc        string
		config      *config.NebulaClusterConfig
		factory     *factoryImpl
		expectedErr bool
	}{
		{
			config: &config.NebulaClusterConfig{
				Namespace:   "nebula-system",
				ClusterName: "nebula",
			},
			factory: &factoryImpl{
				nebulaClusterName:       "",
				nebulaClusterConfigFile: "~/.kube/config1",
				loadingLock:             sync.Mutex{},
				nebulaClusterConfig:     nil,
			},
			expectedErr: true,
		},
		{
			factory: &factoryImpl{
				nebulaClusterName:       "",
				nebulaClusterConfigFile: "~/.kube/config2",
				loadingLock:             sync.Mutex{},
				nebulaClusterConfig:     nil,
			},
			expectedErr: false,
		},
	}

	for i, tc := range testcases {
		if tc.config != nil {
			err := tc.config.SaveToFile(tc.factory.nebulaClusterConfigFile)
			if err != nil {
				t.Error(err)
			}
		}

		_, err := tc.factory.getNebulaClusterConfig()
		if (err != nil) == tc.expectedErr {
			t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n", i, tc.expectedErr, err != nil)
		}
	}
}
