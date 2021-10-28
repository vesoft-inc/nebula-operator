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
	"testing"
)

const template = `
########## authorization ##########
# Enable authorization
--enable_authorize=false

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

--rocksdb_compression_per_level=
`

func TestAppendCustomConfig(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		custom map[string]string
		want   string
	}{
		{
			name: "custom parameter in template",
			data: template,
			custom: map[string]string{
				"enable_authorize": "true",
				"auth_type":        "cloud",
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=true

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=cloud

--rocksdb_compression_per_level=
`,
		},
		{
			name: "custom parameter not in template",
			data: template,
			custom: map[string]string{
				"enable_optimizer": "true",
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=false

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

--rocksdb_compression_per_level=

########## Custom ##########
--enable_optimizer=true
`,
		},
		{
			name: "partial custom parameter in template",
			data: template,
			custom: map[string]string{
				"enable_authorize": "true",
				"enable_optimizer": "true",
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=true

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

--rocksdb_compression_per_level=

########## Custom ##########
--enable_optimizer=true
`,
		},
		{
			name: "multi custom parameter not in template",
			data: template,
			custom: map[string]string{
				"enable_optimizer":     "true",
				"max_log_size":         "100",
				"logtostderr":          "true",
				"log_dir":              "logs",
				"symbolize_stacktrace": "false",
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=false

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

--rocksdb_compression_per_level=

########## Custom ##########
--enable_optimizer=true
--log_dir=logs
--logtostderr=true
--max_log_size=100
--symbolize_stacktrace=false
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendCustomConfig(tt.data, tt.custom); got != tt.want {
				t.Errorf("AppendCustomConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
