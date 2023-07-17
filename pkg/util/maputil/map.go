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

// IsSubMap indicates if the first map is a sub map of the second one
func IsSubMap(first, second map[string]string) bool {
	for k, v := range first {
		if second == nil {
			return false
		}
		if second[k] != v {
			return false
		}
	}
	return true
}

// AllKeysExist indicates whether all the first map keys in the second one
func AllKeysExist(first, second map[string]string) bool {
	for k := range first {
		if second == nil {
			return false
		}
		if _, ok := second[k]; !ok {
			return false
		}
	}
	return true
}

func ResetMap(first, second map[string]string) {
	for k := range first {
		if second == nil {
			return
		}
		if v, ok := second[k]; ok {
			first[k] = v
		}
	}
}

func MergeStringMaps(overwrite bool, ms ...map[string]string) map[string]string {
	n := 0
	for _, m := range ms {
		n += len(m)
	}
	mp := make(map[string]string, n)
	if n == 0 {
		return mp
	}
	for _, m := range ms {
		for k, v := range m {
			if overwrite || !isStringMapExist(mp, k) {
				mp[k] = v
			}
		}
	}
	return mp
}

func isStringMapExist(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, exist := m[key]
	return exist
}
