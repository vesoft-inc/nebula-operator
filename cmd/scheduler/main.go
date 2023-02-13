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

package main

import (
	"flag"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender/predicates"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender/server"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
)

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)
}

func main() {
	var printVersion bool
	var httpPort string

	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.StringVar(&httpPort, "port", "12021", "schedule extender listen address")

	pflag.Parse()
	ctrl.SetLogger(klogr.New())

	if printVersion {
		log.Info("Nebula Operator Version", "version", version.Version())
		os.Exit(0)
	}

	klog.Info("Welcome to Nebula Operator.")
	klog.Info("Nebula Operator Version", "version", version.Version())

	restConfig := ctrl.GetConfigOrDie()

	kubeCli, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Error(err, "failed to get kube config")
		os.Exit(1)
	}

	kubeClient, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "failed to create kube client")
		os.Exit(1)
	}

	predicate := predicates.NewTopology(kubeClient)
	handler := server.NewHandler(extender.NewScheduleExtender(kubeCli, predicate))

	r := gin.Default()
	r.POST("nebula-scheduler/filter", handler.Filter)

	err = r.Run(":" + httpPort)
	if err != nil {
		log.Error(err, "failed to start web server")
		os.Exit(1)
	}
}
