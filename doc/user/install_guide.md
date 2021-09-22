# Install Guide

Instead of manually installing, scaling, upgrading, and uninstalling NebulaGraph in a production environment, you can instead let the Nebula [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) manage the installation for you. This relieves you of the burden of managing different versions and managing the NebulaGraph cluster more easier. Simply update the operator [custom resource (CR)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and the operator controller will apply the corresponding configuration changes for you.

Follow this guide to install Nebula Operator using Helm for in-depth evaluation.

### Requirements

* Kubernetes >= 1.16
* [RBAC](https://kubernetes.io/docs/admin/authorization/rbac) enabled (optional)
* [CoreDNS](https://github.com/coredns/coredns) >= 1.6.0
* [CertManager](https://cert-manager.io) >= 1.2.0
* [OpenKruise](https://openkruise.io) >= 0.8.0
* [Helm](https://helm.sh) >= 3.2.0

## Add-ons

See [add ons](add-ons.md) for how to install the add-ons.

### Get Repo Info

```shell script
helm repo add nebula-operator https://vesoft-inc.github.io/nebula-operator/charts
helm repo update
```

_See [helm repo](https://helm.sh/docs/helm/helm_repo/) for command documentation._

### Install Operator

```shell script
# helm install [NAME] [CHART] [flags]
$ helm install nebula-operator nebula-operator/nebula-operator --namespace=nebula-operator-system --version=${chart_version}
```

Note:   
If the corresponding nebula-system namespace does not exist, you can create the namespace first by running the _kubectl create namespace nebula-operator-system_ command.

${chart_version} represents the chart version of Nebula Operator. For example, v0.8.0. You can view the currently supported versions by running the _helm search repo -l nebula-operator_ command.

_See [configuration](#configure-operator) below._

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._

### Configure Operator

See [Customizing the Chart Before Installing](https://helm.sh/docs/intro/using_helm/#customizing-the-chart-before-installing). To see all configurable options with detailed comments, visit the chart's [values.yaml](https://github.com/vesoft-inc/nebula-operator/blob/master/charts/nebula-operator/values.yaml), or run the following commands:

```shell script
$ helm show values nebula-operator/nebula-operator
```

### Upgrade Operator

If you need to upgrade the Nebula Operator, modify the ${HOME}/nebula-operator/values.yaml file, and then execute the following command to upgrade:

```shell script
$ helm upgrade nebula-operator nebula-operator/nebula-operator -f `${HOME}/nebula-operator/values.yaml`
```

### Uninstall Operator

```shell script
$ helm uninstall nebula-operator
$ kubectl delete crd nebulaclusters.apps.nebula-graph.io
```
