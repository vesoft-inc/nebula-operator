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

package annotation

import "testing"

func TestIsInHaMode(t *testing.T) {
	testCases := []struct {
		name             string
		ann              map[string]string
		expectIsInHaMode bool
	}{
		{
			name:             "nil ann",
			ann:              nil,
			expectIsInHaMode: false,
		}, {
			name:             "empty ann",
			ann:              map[string]string{},
			expectIsInHaMode: false,
		}, {
			name:             "AnnHaModeVal",
			ann:              map[string]string{AnnHaModeKey: AnnHaModeVal},
			expectIsInHaMode: true,
		}, {
			name:             "true",
			ann:              map[string]string{AnnHaModeKey: "true"},
			expectIsInHaMode: true,
		}, {
			name:             "True",
			ann:              map[string]string{AnnHaModeKey: "True"},
			expectIsInHaMode: false,
		}, {
			name:             "TRUE",
			ann:              map[string]string{AnnHaModeKey: "TRUE"},
			expectIsInHaMode: false,
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isInHaMode := IsInHaMode(tc.ann)
			if tc.expectIsInHaMode != isInHaMode {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectIsInHaMode, isInHaMode)
			}
		})
	}
}
