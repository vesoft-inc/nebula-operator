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

package config

import (
	"os"
	"testing"
)

func TestLoadFromFileAndSaveToFile(t *testing.T) {
	testcases := []struct {
		desc          string
		filename      string
		namespace     string
		nebulaCluster string
		config        NebulaClusterConfig
		expected      NebulaClusterConfig
	}{
		{
			desc:     "",
			filename: "",
			config: NebulaClusterConfig{
				ClusterName: "nebula",
				Namespace:   "nebula-system",
			},
			expected: NebulaClusterConfig{
				ClusterName: "nebula",
				Namespace:   "nebula-system",
			},
		},
		{
			desc:     "",
			filename: "",
			config: NebulaClusterConfig{
				ClusterName: "nebula",
				Namespace:   "",
			},
			expected: NebulaClusterConfig{
				ClusterName: "nebula",
				Namespace:   "",
			},
		},
	}

	defer func() {
		for _, tc := range testcases {
			_ = os.Remove(tc.filename)
		}
	}()

	for i, tc := range testcases {
		configFile, err := os.CreateTemp("", "")
		if err != nil {
			t.Error(err)
		}

		tc.filename = configFile.Name()

		if err := tc.config.SaveToFile(tc.filename); err != nil {
			t.Error(err)
		}

		tc.config = NebulaClusterConfig{}

		if err := tc.config.LoadFromFile(tc.filename); err != nil {
			t.Error(err)
		}
		if tc.config != tc.expected {
			t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n", i, tc.expected, tc.config)
		}
	}
}
