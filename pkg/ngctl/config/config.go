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
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

const (
	ncConfigRelativePath = "/.kube/nebulacluster.config"
)

type (
	NebulaClusterConfig struct {
		Namespace   string `json:"namespace,omitempty"`
		ClusterName string `json:"clusterName,omitempty"`
	}
)

func NewNebulaClusterConfig(namespace, clusterName string) *NebulaClusterConfig {
	return &NebulaClusterConfig{
		Namespace:   namespace,
		ClusterName: clusterName,
	}
}

func (c *NebulaClusterConfig) LoadFromFile(filename string) error {
	var err error
	filename, err = c.parseConfigLocation(filename)
	if err != nil {
		return err
	}

	bs, err := ioutil.ReadFile(filename) //nolint: gosec
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(bs, c)
	if err != nil {
		return err
	}
	return nil
}

func (c *NebulaClusterConfig) SaveToFile(filename string) error {
	var err error
	filename, err = c.parseConfigLocation(filename)
	if err != nil {
		return err
	}

	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	content, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, content, 0o600)
}

func (c *NebulaClusterConfig) parseConfigLocation(filename string) (string, error) {
	if filename != "" {
		return filename, nil
	}
	return DefaultConfigLocation()
}

func DefaultConfigLocation() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return usr.HomeDir + ncConfigRelativePath, nil
}
