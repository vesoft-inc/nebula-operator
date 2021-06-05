package config

import (
	"fmt"
	"testing"
)

const template = `
########## authorization ##########
# Enable authorization
--enable_authorize=false

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password
`

func TestAppendCustomConfig(t *testing.T) {
	type args struct {
		data   string
		custom map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "custom parameter in template",
			args: args{
				data: template,
				custom: map[string]string{
					"enable_authorize": "true",
					"auth_type":        "cloud",
				},
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=true

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=cloud
`,
		},
		{
			name: "custom parameter not in template",
			args: args{
				data: template,
				custom: map[string]string{
					"enable_optimizer": "true",
				},
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=false

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

########## Custom ##########
--enable_optimizer=true
`,
		},
		{
			name: "partial custom parameter in template",
			args: args{
				data: template,
				custom: map[string]string{
					"enable_authorize": "true",
					"enable_optimizer": "true",
				},
			},
			want: `
########## authorization ##########
# Enable authorization
--enable_authorize=true

########## Authentication ##########
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

########## Custom ##########
--enable_optimizer=true
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendCustomConfig(tt.args.data, tt.args.custom); got != tt.want {
				fmt.Println(got)
				t.Errorf("AppendCustomConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
