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
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

var paramPattern = regexp.MustCompile(`--(\w+)=(.+)`)

func AppendCustomConfig(data string, custom map[string]string) string {
	if len(custom) == 0 {
		return data
	}

	scanner := bufio.NewScanner(strings.NewReader(data))
	scanner.Split(bufio.ScanLines)

	var b bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, "#") {
			_, _ = b.WriteString(fmt.Sprintf("%s\n", line))
			continue
		}

		if strings.HasPrefix(line, "--") {
			match := paramPattern.FindStringSubmatch(line)
			if len(match) != 3 {
				_, _ = b.WriteString(fmt.Sprintf("%s\n", line))
				continue
			}
			param, value := match[1], match[2]
			v, exists := custom[param]
			if exists {
				value = v
				delete(custom, param)
			}
			_, _ = b.WriteString(fmt.Sprintf("--%s=%s\n", param, value))
		}
	}
	if err := scanner.Err(); err != nil {
		klog.Errorf("reading input failed, error: %v", err)
	}

	if len(custom) > 0 {
		_, _ = b.WriteString("\n########## Custom ##########\n")
	}

	var sortedKeys []string
	for k := range custom {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		_, _ = b.WriteString(fmt.Sprintf("--%s=%s\n", k, custom[k]))
	}

	return b.String()
}
