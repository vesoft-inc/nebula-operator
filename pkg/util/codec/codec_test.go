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
