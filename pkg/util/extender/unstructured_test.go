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

package extender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetSpec(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	res := GetSpec(obj)
	assert.Nil(t, res)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"a": "b",
		},
	}
	target := map[string]interface{}{"a": "b"}
	res = GetSpec(obj)
	assert.Equal(t, target, res)
}

func TestGetTemplateSpec(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	res := GetTemplateSpec(obj)
	assert.Nil(t, res)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"a": "b",
				},
			},
		},
	}
	target := map[string]interface{}{"a": "b"}
	res = GetTemplateSpec(obj)
	assert.Equal(t, target, res)
}

func TestGetStatus(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	status := GetStatus(obj)
	assert.Nil(t, status)

	obj.Object = map[string]interface{}{
		"status": map[string]interface{}{
			"a": "b",
		},
	}
	target := map[string]interface{}{"a": "b"}
	status = GetStatus(obj)
	assert.Equal(t, target, status)
}

func TestGetReplicas(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	replicas := GetReplicas(obj)
	assert.Nil(t, replicas)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(1),
		},
	}
	replicas = GetReplicas(obj)
	assert.Equal(t, int32(1), *replicas)
}

func TestGetContainers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	containers := GetContainers(obj)
	assert.Nil(t, containers)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"c1": "abc",
							"c2": "efg",
						},
					},
				},
			},
		},
	}
	target := []map[string]interface{}{{"c1": "abc", "c2": "efg"}}
	containers = GetContainers(obj)
	assert.Equal(t, target, containers)
}

func TestSetSpecField(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	err := SetSpecField(obj, nil)
	assert.Nil(t, err)
	target := map[string]interface{}{"spec": interface{}(nil)}
	assert.Equal(t, target, obj.Object)

	value := map[string]interface{}{"a": "1", "b": "2"}
	target = map[string]interface{}{"spec": map[string]interface{}{"a": "1", "b": "2"}}
	err = SetSpecField(obj, value)
	assert.Nil(t, err)
	assert.Equal(t, target, obj.Object)
}

func TestSetTemplateAnnotations(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	err := SetTemplateAnnotations(obj, nil)
	assert.Nil(t, err)
	target := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{},
				},
			},
		},
	}
	assert.Equal(t, target, obj.Object)

	value := map[string]string{"a": "1", "b": "2"}
	err = SetTemplateAnnotations(obj, value)
	assert.Nil(t, err)
	// nolint: lll
	anno := obj.Object["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"].(map[string]interface{})["annotations"]
	assert.Equal(t, value["a"], anno.(map[string]interface{})["a"].(string))
}

func TestSetLastAppliedConfigAnnotation(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	err := SetLastAppliedConfigAnnotation(obj)
	assert.Nil(t, err)

	obj.Object = map[string]interface{}{"a": "1", "b": "2"}
	err = SetLastAppliedConfigAnnotation(obj)
	assert.Nil(t, err)
	target := map[string]interface{}{
		"a":        "1",
		"b":        "2",
		"metadata": map[string]interface{}{"annotations": map[string]interface{}{"nebula-graph.io/last-applied-configuration": "null"}}}
	err = SetLastAppliedConfigAnnotation(obj)
	assert.Nil(t, err)
	assert.Equal(t, target, obj.Object)
}

func TestSetUpdatePartition(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	err := SetUpdatePartition(obj, 1, 30, false)
	assert.Nil(t, err)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 2,
		},
	}
	err = SetUpdatePartition(obj, 1, 30, false)
	assert.Nil(t, err)
	target := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 2,
			"updateStrategy": map[string]interface{}{
				"rollingUpdate": map[string]interface{}{
					"partition": int64(1),
				},
				"type": "RollingUpdate",
			},
		},
	}
	assert.Equal(t, target, obj.Object)
}

func TestSetContainerImage(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	err := SetContainerImage(obj, "test", "nebula:v2.0")
	assert.Nil(t, err)

	obj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "nebula:v2.1",
						},
					},
				},
			},
		},
	}
	err = SetContainerImage(obj, "test", "nebula:v2.0")
	assert.Nil(t, err)
	target := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test",
							"image": "nebula:v2.0",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, target, obj.Object)
}

func TestPodTemplateEqual(t *testing.T) {
	newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	oldObj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	equal := PodTemplateEqual(newObj, oldObj)
	assert.Equal(t, false, equal)

	newObj.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"a": "1",
					"b": "2",
				},
			},
		},
	}
	equal = PodTemplateEqual(newObj, oldObj)
	assert.Equal(t, false, equal)

	oldObj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"nebula-graph.io/last-applied-configuration": `{"template":{"spec":{"a":"1","b":"2"}}}`,
			},
		},
	}
	equal = PodTemplateEqual(newObj, oldObj)
	assert.Equal(t, true, equal)
	// nolint: lll
	oldObj.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})["nebula-graph.io/last-applied-configuration"] = `{"template":{"spec":{"a":"1","b":"2","c":"3"}}}`
	equal = PodTemplateEqual(newObj, oldObj)
	assert.Equal(t, false, equal)
}

func TestObjectEqual(t *testing.T) {
	newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	oldObj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	equal := ObjectEqual(newObj, oldObj)
	assert.Equal(t, false, equal)

	newObj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"test.io": "abc",
			},
		},
	}

	oldObj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"nebula-graph.io/last-applied-configuration": `{"replicas":1,"template":{"a":"1","b":"2"},"updateStrategy":{"strategy":"abc"}}`,
				"test.io":  "abc",
				"instance": "efg",
			},
		},
	}
	equal = ObjectEqual(newObj, oldObj)
	assert.Equal(t, false, equal)

	newObj.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})["instance"] = "efg"
	newObj.Object["spec"] = map[string]interface{}{
		"replicas": int64(1),
		"template": map[string]interface{}{
			"a": "1",
			"b": "2",
		},
		"updateStrategy": map[string]interface{}{
			"strategy": "abc",
		},
	}
	equal = ObjectEqual(newObj, oldObj)
	assert.Equal(t, true, equal)
}

func TestIsUpdating(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}

	updating := IsUpdating(obj)
	assert.Equal(t, false, updating)

	// observedGeneration is nil
	obj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": int64(1),
		},
		"spec": map[string]interface{}{
			"replicas": int64(1),
		},
		"status": map[string]interface{}{
			"currentRevision": "796dfdbf86",
			"updateRevision":  "1f6dfdbf213",
		},
	}
	updating = IsUpdating(obj)
	assert.Equal(t, false, updating)

	// currentRevision is nil
	obj.Object["status"] = map[string]interface{}{
		"updateRevision":     "1f6dfdbf213",
		"observedGeneration": int64(1),
	}
	updating = IsUpdating(obj)
	assert.Equal(t, false, updating)

	// updateRevision is nil
	obj.Object["status"] = map[string]interface{}{
		"currentRevision":    "796dfdbf86",
		"observedGeneration": int64(1),
	}
	updating = IsUpdating(obj)
	assert.Equal(t, false, updating)

	// currentRevision equal updateRevision
	obj.Object["status"] = map[string]interface{}{
		"currentRevision":    "796dfdbf86",
		"updateRevision":     "796dfdbf86",
		"observedGeneration": int64(1),
	}
	updating = IsUpdating(obj)
	assert.Equal(t, false, updating)

	obj.Object["metadata"].(map[string]interface{})["generation"] = int64(2)
	obj.Object["status"].(map[string]interface{})["replicas"] = int64(1)
	updating = IsUpdating(obj)
	assert.Equal(t, true, updating)
}
