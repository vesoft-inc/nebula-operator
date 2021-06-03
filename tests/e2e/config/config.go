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
	DefaultKindName                  = "e2e-test"
	DefaultInstallCertManagerVersion = "v1.3.1"
	DefaultInstallKruiseVersion      = "v0.8.1"
)

var TestConfig Config

type Config struct {
	InstallKubernetes         bool
	UninstallKubernetes       bool
	InstallCertManager        bool
	InstallCertManagerVersion string
	InstallKruise             bool
	InstallKruiseVersion      string
	InstallNebulaOperator     bool
	KindName                  string
	KindConfig                string
	StorageClass              string
	NebulaVersion             string
}

func RegisterClusterFlags(flags *flag.FlagSet) {
	flags.BoolVar(&TestConfig.InstallKubernetes, "install-kubernetes", true,
		"If true tests will install kubernetes.")
	flags.BoolVar(&TestConfig.UninstallKubernetes, "uninstall-kubernetes", true,
		"If true tests will uninstall kubernetes. Ignored when --install-kubernetes is false.")
	flags.BoolVar(&TestConfig.InstallCertManager, "install-cert-manager", true,
		"If true tests will install cert-manager.")
	flags.StringVar(&TestConfig.InstallCertManagerVersion, "install-cert-manager-version", DefaultInstallCertManagerVersion,
		"The cert-manager version to install.")
	flags.BoolVar(&TestConfig.InstallKruise, "install-kruise", true,
		"If true tests will install kruise.")
	flags.StringVar(&TestConfig.InstallKruiseVersion, "install-kruise-version", DefaultInstallKruiseVersion,
		"The kruise version to install.")
	flags.BoolVar(&TestConfig.InstallNebulaOperator, "install-nebula-operator", true,
		"If true tests will install nebula operator.")
	flags.StringVar(&TestConfig.KindName, "kind-name", DefaultKindName,
		"The kind name to install.")
	flags.StringVar(&TestConfig.KindConfig, "kind-config", "../../hack/kind-config.yaml",
		"The kind config to install.")
	flags.StringVar(&TestConfig.StorageClass, "storage-class", "",
		"The storage class to use to install nebula cluster."+
			"If don't configure, use the default storage class and then the others in the kubernetes.")
	flags.StringVar(&TestConfig.NebulaVersion, "nebula-version", "v2-nightly",
		"The nebula version.")
}
