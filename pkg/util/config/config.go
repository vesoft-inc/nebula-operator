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
	"bytes"
	"fmt"
	"strings"
)

func AppendCustomConfig(data string, custom map[string]string) string {
	lines := make([]string, 0)
	for k, v := range custom {
		line := fmt.Sprintf("\n--%s=%s\n", k, v)
		lines = append(lines, line)
	}
	config := strings.Join(lines, "")

	var b bytes.Buffer
	_, err := b.WriteString(data)
	if err != nil {
		return data
	}
	_, err = b.WriteString(config)
	if err != nil {
		return data
	}

	return b.String()
}
