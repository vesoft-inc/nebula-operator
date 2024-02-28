/*
Copyright 2023 Vesoft Inc.

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
	"encoding"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v8"
	"github.com/joho/godotenv"
)

var C Config

func init() {
	var envFiles []string
	if files := os.Getenv("DOT_ENV_FILES"); files != "" {
		envFiles = strings.Split(files, ";")
	}

	if err := godotenv.Load(envFiles...); err != nil {
		if len(envFiles) > 0 || !stderrors.Is(err, os.ErrNotExist) {
			panic(err)
		}
		// ignore errors if the default .env file does not exist
	}
	if err := env.Parse(&C); err != nil {
		panic(fmt.Sprintf("parse env config failed, %s", err))
	}
}

type Config struct {
	CommonConfig
	Cluster     ClusterConfig
	Operator    OperatorConfig
	NebulaGraph NebulaClusterConfig
}

type CommonConfig struct {
	// DockerConfigJsonSecret is the docker config file.
	// export E2E_DOCKER_CONFIG_JSON_SECRET=`cat ~/.docker/config.json| base64 -w 0`
	DockerConfigJsonSecret Base64Value `env:"E2E_DOCKER_CONFIG_JSON_SECRET"`
	AWSAccessKey           Base64Value `env:"E2E_AWS_ACCESS_KEY"`
	AWSSecretKey           Base64Value `env:"E2E_AWS_SECRET_KEY"`
	GSSecret               Base64Value `env:"E2E_GCP_SECRET"`
}

type ClusterConfig struct {
	KindConfigPath string `env:"E2E_CLUSTER_KIND_CONFIG_PATH,notEmpty,required" envDefault:"./kind-config.yaml"`
}

type OperatorConfig struct {
	Install   bool   `env:"E2E_OPERATOR_INSTALL,notEmpty,required" envDefault:"true"`
	Namespace string `env:"E2E_OPERATOR_NAMESPACE,notEmpty,required" envDefault:"nebula-operator-system"`
	Name      string `env:"E2E_OPERATOR_NAMESPACE,notEmpty,required" envDefault:"nebula-operator"`
	ChartPath string `env:"E2E_OPERATOR_CHART_PATH,notEmpty,required" envDefault:"../../charts/nebula-operator"`
	Image     string `env:"E2E_OPERATOR_IMAGE"`
}

type NebulaClusterConfig struct {
	ChartPath         string   `env:"E2E_NC_CHART_PATH,notEmpty,required" envDefault:"../../charts/nebula-cluster"`
	Version           string   `env:"E2E_NC_VERSION"`
	AgentImage        string   `env:"E2E_NC_AGENT_IMAGE"`
	AgentVersion      string   `env:"E2E_NC_AGENT_VERSION"`
	GraphdImage       string   `env:"E2E_NC_GRAPHD_IMAGE"`
	MetadImage        string   `env:"E2E_NC_METAD_IMAGE"`
	StoragedImage     string   `env:"E2E_NC_STORAGED_IMAGE"`
	LicenseManagerURL string   `env:"E2E_NC_LICENSE_MANAGER_URL"`
	Zones             []string `env:"E2E_NC_ZONES" envDefault:"zone1,zone2,zone3"`
}

var _ encoding.TextUnmarshaler = (*Base64Value)(nil)

type Base64Value []byte

func (v *Base64Value) UnmarshalText(text []byte) error {
	b, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return err
	}
	*v = b
	return nil
}
