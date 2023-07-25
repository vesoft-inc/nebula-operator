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

package nebula

import (
	"crypto/tls"
	"time"

	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/util/cert"
)

const DefaultTimeout = 10 * time.Second

type Option func(ops *Options)

type Options struct {
	EnableMetaTLS    bool
	EnableClusterTLS bool
	IsStorage        bool
	Timeout          time.Duration
	TLSConfig        *tls.Config
}

func ClientOptions(nc *v1alpha1.NebulaCluster, opts ...Option) ([]Option, error) {
	options := []Option{SetTimeout(DefaultTimeout)}
	if nc.Spec.SSLCerts == nil || (nc.IsGraphdSSLEnabled() && !nc.IsMetadSSLEnabled() && !nc.IsClusterSSLEnabled()) {
		return options, nil
	}

	if nc.IsMetadSSLEnabled() && !nc.IsClusterSSLEnabled() {
		options = append(options, SetMetaTLS(true))
		klog.Infof("cluster [%s/%s] metad SSL enabled", nc.Namespace, nc.Name)
	}
	if nc.IsClusterSSLEnabled() {
		options = append(options, SetClusterTLS(true))
		klog.Infof("cluster [%s/%s] SSL enabled", nc.Namespace, nc.Name)
	}
	caCert, clientCert, clientKey, err := getCerts(nc.Namespace, nc.Spec.SSLCerts)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := cert.LoadTLSConfig(caCert, clientCert, clientKey)
	if err != nil {
		return nil, err
	}
	tlsConfig.InsecureSkipVerify = nc.InsecureSkipVerify()
	tlsConfig.MaxVersion = tls.VersionTLS12
	options = append(options, SetTLSConfig(tlsConfig))
	options = append(options, opts...)
	return options, nil
}

func loadOptions(options ...Option) *Options {
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}
	return opts
}

func SetOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

func SetTimeout(duration time.Duration) Option {
	return func(options *Options) {
		options.Timeout = duration
	}
}

func SetTLSConfig(config *tls.Config) Option {
	return func(options *Options) {
		options.TLSConfig = config
	}
}

func SetMetaTLS(e bool) Option {
	return func(options *Options) {
		options.EnableMetaTLS = e
	}
}

func SetClusterTLS(e bool) Option {
	return func(options *Options) {
		options.EnableClusterTLS = e
	}
}

func SetIsStorage(e bool) Option {
	return func(options *Options) {
		options.IsStorage = e
	}
}
