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

package maputil

import "testing"

func TestIsSubMap(t *testing.T) {
	tests := []struct {
		name   string
		first  map[string]string
		second map[string]string
		want   bool
	}{
		{
			name:   "first map is nil",
			second: map[string]string{"a": "1", "b": "2"},
			want:   true,
		},
		{
			name:   "first map key not exist",
			first:  map[string]string{"c": "1"},
			second: map[string]string{"a": "1", "b": "2"},
			want:   false,
		},
		{
			name:   "first map value not equal",
			first:  map[string]string{"a": "3"},
			second: map[string]string{"a": "1", "b": "2"},
			want:   false,
		},
		{
			name:  "second map is nil",
			first: map[string]string{"a": "1"},
			want:  false,
		},
		{
			name:   "second map not nil",
			first:  map[string]string{"a": "1"},
			second: map[string]string{"a": "1", "b": "2"},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSubMap(tt.first, tt.second); got != tt.want {
				t.Errorf("IsSubMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
