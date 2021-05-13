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
	"flag"
)

const (
	DefaultKindName = "e2e-test"
)

var TestConfig Config

type Config struct {
	InstallKubernetes     bool
	UninstallKubernetes   bool
	InstallCertManager    bool
	InstallKruise         bool
	InstallNebulaOperator bool
	KindName              string
	KindConfig            string
}

func RegisterClusterFlags(flags *flag.FlagSet) {
	flags.BoolVar(&TestConfig.InstallKubernetes, "install-kubernetes", true,
		"If true tests will install kubernetes.")
	flags.BoolVar(&TestConfig.UninstallKubernetes, "uninstall-kubernetes", true,
		"If true tests will uninstall kubernetes. Ignored when --install-kubernetes is false.")
	flags.BoolVar(&TestConfig.InstallCertManager, "install-cert-manager", true,
		"If true tests will install cert-manager.")
	flags.BoolVar(&TestConfig.InstallKruise, "install-kruise", true,
		"If true tests will install kruise.")
	flags.BoolVar(&TestConfig.InstallNebulaOperator, "install-nebula-operator", true,
		"If true tests will install nebula operator.")
	flags.StringVar(&TestConfig.KindName, "kind-name", DefaultKindName,
		"The kind name to install.")
	flags.StringVar(&TestConfig.KindConfig, "kind-config", "../../hack/kind-config.yaml",
		"The kind config to install.")
}
