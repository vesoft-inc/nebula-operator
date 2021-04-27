package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender/predicates"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender/server"
	"github.com/vesoft-inc/nebula-operator/pkg/version"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appsv1alpha1.AddToScheme(scheme)
}

func main() {
	var printVersion bool
	var httpPort string

	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.StringVar(&httpPort, "port", "12021", "schedule extender listen address")
	flag.Parse()

	if printVersion {
		fmt.Printf("Nebula Operator Version: %#v\n", version.Version())
		os.Exit(0)
	}
	klog.Info("Welcome to Nebula Operator.")
	klog.Infof("Nebula Operator Version: %#v", version.Version())

	restConfig := ctrl.GetConfigOrDie()

	kubeCli, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	kubeClient, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	predicate := predicates.NewTopology(kubeClient)
	handler := server.NewHandler(extender.NewScheduleExtender(kubeCli, predicate))

	r := gin.Default()
	r.POST("nebula-scheduler/filter", handler.Filter)

	_ = r.Run(":" + httpPort)
}
