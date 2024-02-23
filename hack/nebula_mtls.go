/*
Copyright 2024 Vesoft Inc.

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

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	ng "github.com/vesoft-inc/nebula-go/v3"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/util/cert"
)

var service string
var servicePort int
var clusterName string
var namespace string
var enableTLS bool
var insecureSkipVerify bool
var serverName string

func main() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.StringVar(&service, "service", "", "nebula service type, value graph, meta, storage is valid")
	pflag.IntVar(&servicePort, "service-port", 0, "nebula service port")
	pflag.StringVar(&clusterName, "cluster-name", "", "nebula cluster name")
	pflag.StringVar(&namespace, "namespace", "default", "cluster namespace")
	pflag.BoolVar(&enableTLS, "enable-tls", false, "connect to nebula service enable mTLS")
	pflag.BoolVar(&insecureSkipVerify, "insecure-skip-verify", false, "a client verifies the server's certificate chain and host name")
	pflag.StringVar(&serverName, "server-name", "", "server name is used to verify the hostname on the returned certificates")
	pflag.Parse()

	var tlsConfig *tls.Config
	var err error
	if enableTLS {
		tlsConfig, err = getTLSConfig()
		if err != nil {
			klog.Fatal(err)
		}
	}

	switch service {
	case "graph":
		hostList := []ng.HostAddress{
			{
				Host: fmt.Sprintf("%s-graphd-svc.%s.svc.cluster.local", clusterName, namespace),
				Port: servicePort,
			},
		}

		var pool *ng.ConnectionPool
		if enableTLS {
			pool, err = ng.NewSslConnectionPool(hostList, ng.GetDefaultConf(), tlsConfig, ng.DefaultLogger{})
		} else {
			pool, err = ng.NewConnectionPool(hostList, ng.GetDefaultConf(), ng.DefaultLogger{})
		}
		if err != nil {
			klog.Fatal(err)
		}
		defer pool.Close()

		session, err := pool.GetSession("root", "nebula")
		if err != nil {
			klog.Fatal(err)
		}
		defer session.Release()

		klog.Info("get session succeeded")
	case "meta":
		meta0 := []string{fmt.Sprintf("%s-metad-0.%s-metad-headless.%s.svc.cluster.local:%d",
			clusterName, clusterName, namespace, servicePort)}
		options := []nebula.Option{nebula.SetTimeout(nebula.DefaultTimeout), nebula.SetIsMeta(true)}
		if enableTLS {
			options = append(options, nebula.SetMetaTLS(true), nebula.SetTLSConfig(tlsConfig))
		}
		mc, err := nebula.NewMetaClient(meta0, options...)
		if err != nil {
			klog.Fatal(err)
		}
		defer mc.Disconnect()

		hosts, err := mc.ListHosts(meta.ListHostType_STORAGE)
		if err != nil {
			klog.Fatal(err)
		}
		for _, host := range hosts {
			klog.Infof("storage host: %s", host.HostAddr.String())
		}
	case "storage":
		host0 := []string{fmt.Sprintf("%s-storaged-0.%s-storaged-headless.%s.svc.cluster.local:%d",
			clusterName, clusterName, namespace, servicePort)}
		options := []nebula.Option{nebula.SetTimeout(nebula.DefaultTimeout), nebula.SetIsStorage(true)}
		if enableTLS {
			options = append(options, nebula.SetStorageTLS(true), nebula.SetTLSConfig(tlsConfig))
		}
		sc, err := nebula.NewStorageClient(host0, options...)
		if err != nil {
			klog.Fatal(err)
		}
		defer sc.Disconnect()

		leaderParts, err := sc.GetLeaderParts()
		if err != nil {
			klog.Fatal(err)
		}
		if len(leaderParts) == 0 {
			klog.Info("spaces not found")
		}
		for spaceId, parts := range leaderParts {
			klog.Infof("space %d, parts %v", spaceId, parts)
		}
	default:
		panic("unknown service type")
	}
}

func getTLSConfig() (*tls.Config, error) {
	caCert, clientCert, clientKey, err := getCerts()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := cert.LoadTLSConfig(caCert, clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("load tls config failed: %v", err)
	}
	tlsConfig.InsecureSkipVerify = insecureSkipVerify
	tlsConfig.ServerName = serverName
	tlsConfig.MaxVersion = tls.VersionTLS12
	return tlsConfig, nil
}

func getCerts() ([]byte, []byte, []byte, error) {
	if os.Getenv("CA_CERT_PATH") != "" &&
		os.Getenv("CLIENT_CERT_PATH") != "" &&
		os.Getenv("CLIENT_KEY_PATH") != "" {
		caCert, err := os.ReadFile(os.Getenv("CA_CERT_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		clientCert, err := os.ReadFile(os.Getenv("CLIENT_CERT_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		clientKey, err := os.ReadFile(os.Getenv("CLIENT_KEY_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		return caCert, clientCert, clientKey, nil
	}
	return nil, nil, nil, errors.New("cert environments not found")
}
