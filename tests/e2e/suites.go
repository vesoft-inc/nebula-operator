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

package e2e

import (
	"bufio"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	e2econfig "github.com/vesoft-inc/nebula-operator/tests/e2e/config"
)

func setupSuite() {
	framework.Logf("running BeforeSuite actions on node 1")

	ginkgo.By("Add NebulaCluster scheme")
	err := v1alpha1.AddToScheme(scheme.Scheme)
	framework.ExpectNoError(err, "failed to add NebulaCluster scheme")

	ginkgo.By("Setup kubernetes")
	setupKubernetes()

	ginkgo.By("Setup cert manager")
	setupCertManager()

	ginkgo.By("Setup kruise")
	setupKruise()

	ginkgo.By("Setup nebula operator")
	setupNebulaOperator()
}

func setupSuitePerGinkgoNode() {
	framework.Logf("running BeforeSuite actions on all nodes")
}

func cleanupSuite() {
	framework.Logf("running AfterSuite actions on all nodes")
	framework.RunCleanupActions()

	ginkgo.By("Cleanup kubernetes")
	cleanupKubernetes()
}

func cleanupSuitePerGinkgoNode() {
	framework.Logf("running AfterSuite actions on node 1")
}

func setupKubernetes() {
	if !e2econfig.TestConfig.InstallKubernetes {
		return
	}
	framework.Logf("install kubernetes")

	kindName := e2econfig.TestConfig.KindName
	if kindName == "" {
		kindName = e2econfig.DefaultKindName
	}
	stdout, _, err := framework.RunCmd("kind", "get", "clusters")
	framework.ExpectNoError(err, "failed to get kind clusters")
	isInstall := false
	sc := bufio.NewScanner(strings.NewReader(stdout))
	for sc.Scan() {
		if sc.Text() == kindName {
			isInstall = true
		}
	}
	if err := sc.Err(); err != nil {
		framework.ExpectNoError(err)
	}

	if !isInstall {
		cmd := []string{"kind", "create", "cluster", "--name", kindName}
		if framework.TestContext.KubeConfig != "" {
			cmd = append(cmd, "--kubeconfig", framework.TestContext.KubeConfig)
		}
		if e2econfig.TestConfig.KindConfig != "" {
			cmd = append(cmd, "--config", e2econfig.TestConfig.KindConfig)
		}
		_, _, err := framework.RunCmd(cmd[0], cmd[1:]...)
		framework.ExpectNoError(err, "failed to install kubernetes %s", kindName)
	}

	clientConfig, err := framework.LoadConfig()
	framework.ExpectNoError(err, "failed load config")
	runtimeClient, err := client.New(clientConfig, client.Options{})
	framework.ExpectNoError(err)

	err = waitForKubernetesWorkloadsReady("", 30*time.Minute, 10*time.Second, runtimeClient)
	framework.ExpectNoError(err, "failed to wait for kubernetes workloads ready")
}

func setupCertManager() {
	if !e2econfig.TestConfig.InstallCertManager {
		return
	}

	helmName := "cert-manager" // nolint: goconst
	helmNamespace := "cert-manager"
	workloadNamespaces := []string{helmNamespace}
	helmArgs := []string{
		"install", helmName,
		"cert-manager",
		"--repo", "https://charts.jetstack.io",
		"--namespace", helmNamespace,
		"--create-namespace",
		"--version", e2econfig.TestConfig.InstallCertManagerVersion,
		"--set", "installCRDs=true",
	}
	helmInstall(helmName, helmNamespace, workloadNamespaces, helmArgs...)
}

func setupKruise() {
	if !e2econfig.TestConfig.InstallKruise {
		return
	}
	helmName := "kruise"
	helmNamespace := "default"
	workloadNamespaces := []string{"kruise-system"}
	helmArgs := []string{
		"install", helmName,
		fmt.Sprintf("https://github.com/openkruise/kruise/releases/download/%s/kruise-chart.tgz",
			e2econfig.TestConfig.InstallKruiseVersion),
	}
	helmInstall(helmName, helmNamespace, workloadNamespaces, helmArgs...)
}

func setupNebulaOperator() {
	if !e2econfig.TestConfig.InstallNebulaOperator {
		return
	}
	helmName := "nebula-operator"
	helmNamespace := "nebula-operator-system"
	workloadNamespaces := []string{helmNamespace}
	helmArgs := []string{
		"install", helmName,
		path.Join(framework.TestContext.RepoRoot, "charts/nebula-operator"),
		"--namespace", helmNamespace,
		"--create-namespace",
	}
	helmInstall(helmName, helmNamespace, workloadNamespaces, helmArgs...)
}

func cleanupKubernetes() {
	if !e2econfig.TestConfig.InstallKubernetes || !e2econfig.TestConfig.UninstallKubernetes {
		return
	}
	framework.Logf("uninstall kubernetes")

	kindName := e2econfig.TestConfig.KindName
	if kindName == "" {
		kindName = e2econfig.DefaultKindName
	}
	_, _, err := framework.RunCmd("kind", "delete", "cluster", "--name", kindName)
	framework.ExpectNoError(err, "failed to uninstall kubernetes")
}

func isHelmInstalled(name, namespace string) bool {
	args := []string{"status", name, "--namespace", namespace}

	if framework.TestContext.KubeConfig != "" {
		args = append(args, "--kubeconfig", framework.TestContext.KubeConfig)
	}

	_, _, err := framework.RunCmd("helm", args...)
	if err != nil {
		if !strings.Contains(err.Error(), "release: not found") {
			framework.ExpectNoError(err, "failed to helm status %s%s", namespace, name)
		}
		return false
	}
	return true
}

func helmInstall(name, namespace string, workloadNamespaces []string, args ...string) {
	framework.Logf("install %s", name)
	if !isHelmInstalled(name, namespace) {
		if framework.TestContext.KubeConfig != "" {
			args = append(args, "--kubeconfig", framework.TestContext.KubeConfig)
		}
		_, _, err := framework.RunCmd("helm", args...)
		framework.ExpectNoError(err, "failed to install helm %s/%s", namespace, name)
	} else {
		framework.Logf("helm %s/%s already installed", namespace, name)
	}

	clientConfig, err := framework.LoadConfig()
	framework.ExpectNoError(err, "failed load config")
	runtimeClient, err := client.New(clientConfig, client.Options{})
	framework.ExpectNoError(err)

	for _, ns := range workloadNamespaces {
		err = waitForKubernetesWorkloadsReady(ns, 20*time.Minute, 5*time.Second, runtimeClient)
		framework.ExpectNoError(err, "failed to wait for %s/%s ready", ns, name)
	}
}

func waitForKubernetesWorkloadsReady(namespace string, timeout, pollInterval time.Duration, runtimeClient client.Client) error {
	start := time.Now()
	listOptions := client.ListOptions{}
	nsStr := "all namespace"
	if namespace != "" {
		listOptions.Namespace = namespace
		nsStr = fmt.Sprintf("namespace %q", namespace)
	}

	getNotReadyKeys := func(list client.ObjectList) ([]string, string, error) {
		err := runtimeClient.List(context.TODO(), list, &listOptions)
		if err != nil {
			return nil, "", err
		}
		var notReady []string
		switch v := list.(type) {
		case *appsv1.DeploymentList:
			for i := range v.Items {
				item := v.Items[i]
				if item.Status.Replicas != item.Status.ReadyReplicas {
					notReady = append(notReady, client.ObjectKeyFromObject(&item).String())
				}
			}
			return notReady, "Deployment", nil
		case *appsv1.StatefulSetList:
			for i := range v.Items {
				item := v.Items[i]
				if item.Status.Replicas != item.Status.ReadyReplicas {
					notReady = append(notReady, client.ObjectKeyFromObject(&item).String())
				}
			}
			return notReady, "StatefulSet", nil
		case *appsv1.DaemonSetList:
			for i := range v.Items {
				item := v.Items[i]
				if item.Status.DesiredNumberScheduled != item.Status.NumberReady {
					notReady = append(notReady, client.ObjectKeyFromObject(&item).String())
				}
			}
			return notReady, "DaemonSet", nil
		}
		return nil, "", fmt.Errorf("unkonw ObjectList %T", list)
	}

	objectLists := []client.ObjectList{
		&appsv1.DeploymentList{},
		&appsv1.StatefulSetList{},
		&appsv1.DaemonSetList{},
	}

	return wait.PollImmediate(pollInterval, timeout, func() (bool, error) {
		success := true
		framework.Logf("waiting up to %v for workloads in %s to ready", timeout, nsStr)
		for _, list := range objectLists {
			notReady, kind, err := getNotReadyKeys(list)
			if err != nil {
				framework.Logf("failed get %q in %s: %v", kind, nsStr, err)
				success = false
			}
			if len(notReady) > 0 {
				framework.Logf("there are not ready %q in %s: %v (%d seconds elapsed)",
					kind, nsStr, notReady, int(time.Since(start).Seconds()))
				success = false
			}
		}
		return success, nil
	})
}
