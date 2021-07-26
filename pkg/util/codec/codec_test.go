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

package codec

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestEncode(t *testing.T) {
	type x interface{}
	tests := []struct {
		name    string
		obj     interface{}
		want    string
		wantErr bool
	}{
		{
			name:    "object is nil",
			want:    "null",
			wantErr: false,
		},
		{
			name: "object is struct",
			obj: struct {
				T1 string
				T2 int
			}{
				T1: "abc",
				T2: 123,
			},
			want:    `{"T1":"abc","T2":123}`,
			wantErr: false,
		},
		{
			name:    "object is map",
			obj:     map[string]string{"T1": "abc", "T2": "123"},
			want:    `{"T1":"abc","T2":"123"}`,
			wantErr: false,
		},
		{
			name:    "object is slice",
			obj:     []string{"a", "b", "c"},
			want:    `["a","b","c"]`,
			wantErr: false,
		},
		{
			name:    "object is array",
			obj:     [3]int{1, 2, 3},
			want:    "[1,2,3]",
			wantErr: false,
		},
		{
			name:    "object is pointer",
			obj:     (*int)(unsafe.Pointer(reflect.ValueOf(new(x)).Pointer())),
			want:    `0`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Encode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
