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

package util

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/config"
)

type (
	Factory interface {
		kcmdutil.Factory

		AddFlags(cmd *cobra.Command)
		ToRuntimeClient() (client.Client, error)
		GetNebulaClusterNameAndNamespace(withUseConfig bool, args []string) (string, string, error)
		GetNebulaClusterNamesAndNamespace(withUseConfig bool, args []string) ([]string, string, error)
		// GetNebulaClusterName() (string, error)
		// GetNebulaClusterNameWithoutConfig() string
		// GetNamespace() (string, error)
		GetNebulaClusterConfigFile() (string, error)
	}
	factoryImpl struct {
		kcmdutil.Factory

		nebulaClusterName       string
		nebulaClusterConfigFile string

		loadingLock         sync.Mutex
		nebulaClusterConfig *config.NebulaClusterConfig
	}
)

var (
	_               Factory = (*factoryImpl)(nil)
	errNotSpecified         = errors.New("Not Specified")
)

func NewFactory(clientGetter genericclioptions.RESTClientGetter) Factory {
	return &factoryImpl{
		Factory: kcmdutil.NewFactory(clientGetter),
	}
}

func IsErNotSpecified(err error) bool {
	return err == errNotSpecified
}

func (f *factoryImpl) AddFlags(cmd *cobra.Command) {
	location, _ := config.DefaultConfigLocation()
	cmd.Flags().StringVar(&f.nebulaClusterName, "nebulacluster", "", "Specify the nebula cluster.")
	cmd.Flags().StringVar(&f.nebulaClusterConfigFile, "config", location, "Specify the nebula cluster config.")
}

func (f *factoryImpl) ToRuntimeClient() (client.Client, error) {
	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return client.New(restConfig, client.Options{})
}

func (f *factoryImpl) GetNamespace() (string, error) {
	namespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return "", err
	}

	if enforceNamespace {
		return namespace, err
	}

	c, err := f.getNebulaClusterConfig()
	if err != nil {
		return namespace, nil
	}

	return c.Namespace, err
}

func (f *factoryImpl) GetNebulaClusterNameAndNamespace(withUseConfig bool, args []string) (name, namespace string, err error) {
	var names []string
	names, namespace, err = f.GetNebulaClusterNamesAndNamespace(withUseConfig, args)
	if len(names) > 0 {
		name = names[0]
	}
	return name, namespace, err
}

func (f *factoryImpl) GetNebulaClusterNamesAndNamespace(withUseConfig bool, args []string) (names []string, namespace string, err error) {
	var enforceNamespace bool
	namespace, enforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", err
	}

	if f.nebulaClusterName != "" {
		return []string{f.nebulaClusterName}, namespace, nil
	}

	if len(args) > 0 {
		return args, namespace, nil
	}

	if enforceNamespace || !withUseConfig {
		return nil, namespace, errNotSpecified
	}

	c, err := f.getNebulaClusterConfig()
	if err != nil {
		return nil, "", errNotSpecified
	}

	return []string{c.ClusterName}, c.Namespace, nil
}

func (f *factoryImpl) GetNebulaClusterName() (string, error) {
	return f.getNebulaClusterName(true)
}

func (f *factoryImpl) GetNebulaClusterNameWithoutConfig() string {
	name, _ := f.getNebulaClusterName(false)
	return name
}

func (f *factoryImpl) getNebulaClusterName(withConfig bool) (string, error) {
	if !withConfig || f.nebulaClusterName != "" {
		return f.nebulaClusterName, nil
	}

	c, err := f.getNebulaClusterConfig()
	if err != nil {
		return "", err
	}

	return c.ClusterName, nil
}

func (f *factoryImpl) GetNebulaClusterConfigFile() (string, error) {
	if f.nebulaClusterConfigFile != "" {
		return f.nebulaClusterConfigFile, nil
	}
	return config.DefaultConfigLocation()
}

func (f *factoryImpl) getNebulaClusterConfig() (*config.NebulaClusterConfig, error) {
	if f.nebulaClusterConfig != nil {
		return f.nebulaClusterConfig, nil
	}

	f.loadingLock.Lock()
	defer f.loadingLock.Unlock()
	if f.nebulaClusterConfig != nil {
		return f.nebulaClusterConfig, nil
	}

	c := &config.NebulaClusterConfig{}
	if err := c.LoadFromFile(f.nebulaClusterConfigFile); err != nil {
		return nil, errors.Wrap(err, "unable to load nebula cluster config")
	}

	f.nebulaClusterConfig = c

	return c, nil
}
