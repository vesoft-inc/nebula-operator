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

package webhook

import (
	"context"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.Handler = &testHandler{}

type testHandler struct{ string }

func (h *testHandler) Handle(context.Context, admission.Request) admission.Response {
	return admission.Response{}
}

func Test_RegisterHandlers(t *testing.T) {
	a := &testHandler{"a"}
	a1 := &testHandler{"a1"}
	b := &testHandler{"b"}
	b1 := &testHandler{"b1"}
	c := &testHandler{"c"}
	d := &testHandler{"d"}
	e := &testHandler{"e"}
	testCases := []struct {
		name             string
		HandlerMap       map[string]admission.Handler
		m                map[string]admission.Handler
		expectHandlerMap map[string]admission.Handler
	}{
		{
			name: "all",
			HandlerMap: map[string]admission.Handler{
				"/a": a,
				"/b": b,
				"/c": c,
				"/e": e,
			},
			m: map[string]admission.Handler{
				"/a": a1,
				"b":  b1,
				"/c": c,
				"/d": d,
			},
			expectHandlerMap: map[string]admission.Handler{
				"/a": a1,
				"/b": b1,
				"/c": c,
				"/d": d,
				"/e": e,
			},
		},
	}
	hm := HandlerMap
	defer func() {
		HandlerMap = hm
	}()
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			HandlerMap = tc.HandlerMap
			registerHandlers(tc.m)

			if !reflect.DeepEqual(tc.expectHandlerMap, HandlerMap) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectHandlerMap, HandlerMap)
			}
		})
	}
}
