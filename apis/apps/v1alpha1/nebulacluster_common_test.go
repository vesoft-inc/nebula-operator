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

package v1alpha1

import (
	"reflect"
	"testing"
)

func Test_mergeStringMaps(t *testing.T) {
	testCases := []struct {
		name      string
		overwrite bool
		ms        []map[string]string
		expectMap map[string]string
	}{
		{
			name:      "nil",
			overwrite: false,
			ms:        nil,
			expectMap: map[string]string{},
		}, {
			name:      "nil maps",
			overwrite: false,
			ms:        []map[string]string{nil, nil, nil},
			expectMap: map[string]string{},
		}, {
			name:      "empty maps",
			overwrite: false,
			ms:        []map[string]string{{}, {}, {}},
			expectMap: map[string]string{},
		}, {
			name:      "nil/empty maps",
			overwrite: false,
			ms:        []map[string]string{nil, {}, nil, {}, {}, nil},
			expectMap: map[string]string{},
		}, {
			name:      "not overwrite",
			overwrite: false,
			ms: []map[string]string{
				nil, {
					"k1": "v1",
					"k2": "v2",
				}, {}, nil, {
					"k1": "v11",
					"k3": "v3",
				}, nil, {},
			},
			expectMap: map[string]string{
				"k1": "v1",
				"k2": "v2",
				"k3": "v3",
			},
		}, {
			name:      "overwrite",
			overwrite: true,
			ms: []map[string]string{
				{}, {
					"k1": "v1",
					"k2": "v2",
				}, nil, {}, {
					"k1": "v11",
					"k3": "v3",
				}, {}, nil,
			},
			expectMap: map[string]string{
				"k1": "v11",
				"k2": "v2",
				"k3": "v3",
			},
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeStringMaps(tc.overwrite, tc.ms...)
			if !reflect.DeepEqual(tc.expectMap, got) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectMap, got)
			}
		})
	}
}

func Test_isStringMapExist(t *testing.T) {
	testCases := []struct {
		name        string
		m           map[string]string
		key         string
		expectExist bool
	}{
		{
			name:        "nil",
			m:           nil,
			key:         "k",
			expectExist: false,
		}, {
			name:        "empty",
			m:           map[string]string{},
			key:         "k",
			expectExist: false,
		}, {
			name:        "not exist",
			m:           map[string]string{"k": ""},
			key:         "not exist",
			expectExist: false,
		}, {
			name:        "exist",
			m:           map[string]string{"k": ""},
			key:         "k",
			expectExist: true,
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isStringMapExist(tc.m, tc.key)
			if !reflect.DeepEqual(tc.expectExist, got) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectExist, got)
			}
		})
	}
}
