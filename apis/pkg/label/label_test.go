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

package label

import (
	"reflect"
	"testing"
)

func TestCluster(t *testing.T) {
	testCases := []struct {
		name        string
		baseLabel   Label
		clusterName string
		expectLabel Label
	}{
		{
			name:        "empty string",
			baseLabel:   Label{},
			clusterName: "",
			expectLabel: Label{ClusterLabelKey: ""},
		},
		{
			name:        "a",
			baseLabel:   Label{},
			clusterName: "a",
			expectLabel: Label{ClusterLabelKey: "a"},
		},
		{
			name:        "extra",
			baseLabel:   Label{"extra": "extra"},
			clusterName: "a",
			expectLabel: Label{ClusterLabelKey: "a", "extra": "extra"},
		},
		{
			name:        "cover",
			baseLabel:   Label{ClusterLabelKey: "a"},
			clusterName: "b",
			expectLabel: Label{ClusterLabelKey: "b"},
		},
		{
			name:        "cover with extra",
			baseLabel:   Label{ClusterLabelKey: "a", "extra": "extra"},
			clusterName: "b",
			expectLabel: Label{ClusterLabelKey: "b", "extra": "extra"},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.baseLabel.Cluster(tc.clusterName)
			if !reflect.DeepEqual(tc.expectLabel, l) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectLabel, l)
			}
		})
	}
}

func TestComponent(t *testing.T) {
	testCases := []struct {
		name          string
		baseLabel     Label
		componentName string
		expectLabel   Label
	}{
		{
			name:          "empty string",
			baseLabel:     Label{},
			componentName: "",
			expectLabel:   Label{ComponentLabelKey: ""},
		},
		{
			name:          "a",
			baseLabel:     Label{},
			componentName: "a",
			expectLabel:   Label{ComponentLabelKey: "a"},
		},
		{
			name:          "extra",
			baseLabel:     Label{"extra": "extra"},
			componentName: "a",
			expectLabel:   Label{ComponentLabelKey: "a", "extra": "extra"},
		},
		{
			name:          "cover",
			baseLabel:     Label{ComponentLabelKey: "a"},
			componentName: "b",
			expectLabel:   Label{ComponentLabelKey: "b"},
		},
		{
			name:          "cover with extra",
			baseLabel:     Label{ComponentLabelKey: "a", "extra": "extra"},
			componentName: "b",
			expectLabel:   Label{ComponentLabelKey: "b", "extra": "extra"},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.baseLabel.Component(tc.componentName)
			if !reflect.DeepEqual(tc.expectLabel, l) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectLabel, l)
			}
		})
	}
}

func TestGraphd_Metad_Storaged(t *testing.T) {
	testCases := []struct {
		name        string
		baseLabel   Label
		fn          func(l Label) Label
		expectLabel Label
	}{
		{
			name:        "empty label",
			baseLabel:   Label{},
			fn:          func(l Label) Label { return l.Graphd() },
			expectLabel: Label{ComponentLabelKey: GraphdLabelVal},
		},
		{
			name:        "cover",
			baseLabel:   Label{ComponentLabelKey: "a"},
			fn:          func(l Label) Label { return l.Graphd() },
			expectLabel: Label{ComponentLabelKey: GraphdLabelVal},
		},
		{
			name:        "empty label",
			baseLabel:   Label{},
			fn:          func(l Label) Label { return l.Metad() },
			expectLabel: Label{ComponentLabelKey: MetadLabelVal},
		},
		{
			name:        "cover",
			baseLabel:   Label{ComponentLabelKey: "a"},
			fn:          func(l Label) Label { return l.Metad() },
			expectLabel: Label{ComponentLabelKey: MetadLabelVal},
		},
		{
			name:        "empty label",
			baseLabel:   Label{},
			fn:          func(l Label) Label { return l.Storaged() },
			expectLabel: Label{ComponentLabelKey: StoragedLabelVal},
		},
		{
			name:        "cover",
			baseLabel:   Label{ComponentLabelKey: "a"},
			fn:          func(l Label) Label { return l.Storaged() },
			expectLabel: Label{ComponentLabelKey: StoragedLabelVal},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.fn(tc.baseLabel)
			if !reflect.DeepEqual(tc.expectLabel, l) {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expectLabel, l)
			}
		})
	}
}

func TestIsNebulaComponent(t *testing.T) {
	testCases := []struct {
		name   string
		label  Label
		expect bool
	}{
		{
			name:   "nil label",
			label:  nil,
			expect: false,
		},
		{
			name:   "empty label",
			label:  Label{},
			expect: false,
		},
		{
			name:   "not exists key",
			label:  Label{"not exists": GraphdLabelVal},
			expect: false,
		},
		{
			name:   "not exists val",
			label:  Label{ComponentLabelKey: "not exists"},
			expect: false,
		},
		{
			name:   "GraphdLabelVal",
			label:  Label{ComponentLabelKey: GraphdLabelVal},
			expect: true,
		},
		{
			name:   "MetadLabelVal",
			label:  Label{ComponentLabelKey: MetadLabelVal},
			expect: true,
		},
		{
			name:   "StoragedLabelVal",
			label:  Label{ComponentLabelKey: StoragedLabelVal},
			expect: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.label.IsNebulaComponent()
			if tc.expect != b {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expect, b)
			}
		})
	}
}

func TestIsManagedByNebulaOperator(t *testing.T) {
	testCases := []struct {
		name   string
		label  Label
		expect bool
	}{
		{
			name:   "nil label",
			label:  nil,
			expect: false,
		},
		{
			name:   "empty label",
			label:  Label{},
			expect: false,
		},
		{
			name:   "not exists key",
			label:  Label{"not exists": NebulaOperator},
			expect: false,
		},
		{
			name:   "not exists val",
			label:  Label{ManagedByLabelKey: "not exists"},
			expect: false,
		},
		{
			name:   "NebulaOperator",
			label:  Label{ManagedByLabelKey: NebulaOperator},
			expect: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.label.IsManagedByNebulaOperator()
			if tc.expect != b {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, tc.expect, b)
			}
		})
	}
}

func TestIsGraphd_(t *testing.T) {
	testCases := []struct {
		name             string
		label            Label
		expectIsGraphd   bool
		expectIsMetad    bool
		expectIsStoraged bool
	}{
		{
			name:             "nil label",
			label:            nil,
			expectIsGraphd:   false,
			expectIsMetad:    false,
			expectIsStoraged: false,
		},
		{
			name:             "empty label",
			label:            Label{},
			expectIsGraphd:   false,
			expectIsMetad:    false,
			expectIsStoraged: false,
		},
		{
			name:             "not exists key",
			label:            Label{"not exists": GraphdLabelVal},
			expectIsGraphd:   false,
			expectIsMetad:    false,
			expectIsStoraged: false,
		},
		{
			name:             "not exists val",
			label:            Label{ComponentLabelKey: "not exists"},
			expectIsGraphd:   false,
			expectIsMetad:    false,
			expectIsStoraged: false,
		},
		{
			name:             "GraphdLabelVal",
			label:            Label{ComponentLabelKey: GraphdLabelVal},
			expectIsGraphd:   true,
			expectIsMetad:    false,
			expectIsStoraged: false,
		},
		{
			name:             "MetadLabelVal",
			label:            Label{ComponentLabelKey: MetadLabelVal},
			expectIsGraphd:   false,
			expectIsMetad:    true,
			expectIsStoraged: false,
		},
		{
			name:             "StoragedLabelVal",
			label:            Label{ComponentLabelKey: StoragedLabelVal},
			expectIsGraphd:   false,
			expectIsMetad:    false,
			expectIsStoraged: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := [3]bool{tc.label.IsGraphd(), tc.label.IsMetad(), tc.label.IsStoraged()}
			if expect := [3]bool{tc.expectIsGraphd, tc.expectIsMetad, tc.expectIsStoraged}; expect != actual {
				t.Errorf("%d: Expected: \n%#v\n but actual: \n%#v\n",
					i, expect, actual)
			}
		})
	}
}
