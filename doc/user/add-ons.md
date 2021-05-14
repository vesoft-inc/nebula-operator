# Installing Add-ons

**Caution:**
This section links to third party projects that provide functionality required by nebula-operator. The nebula-operator project authors aren't responsible for these projects.

## coredns
[CoreDNS](https://coredns.io/) is a flexible, extensible DNS server which can be [installed](https://github.com/coredns/deployment/tree/master/kubernetes) as the in-cluster DNS for pods.

## cert-manager
[cert-manager](https://cert-manager.io/) is a tool that automates certificate management. It makes use of extending the Kubernetes API server using a Webhook server to provide dynamic admission control over cert-manager resources. 

Consult the [cert-manager installation documentation](https://cert-manager.io/docs/installation/kubernetes/) to get started.

## openkruise
**Note:**
nebula-operator need advanced features for StatefulSet when it starts.

[openkruise](https://openkruise.io/en-us/)  is a full set of standard extensions for Kubernetes. It works well with original Kubernetes and provides more powerful and efficient features for managing applications Pods, sidecar containers, and even images on Node.

Consult the [openkruise installation documentation](https://openkruise.io/en-us/docs/installation.html) to get started.

## sig-storage-local-static-provisioner
**Note:**
Only you deploy NebulaGraph with local storage when it is required.

[local-static-provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner) manages the PersistentVolume lifecycle for pre-allocated disks by detecting and creating PVs for each local disk on the host, and cleaning up the disks when released. It does not support dynamic provisioning.

Follow the [getting started guide](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/docs/getting-started.md) to deploy local-volume-provisioner to provision local volumes.

Follow the [best practices](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/docs/best-practices.md) for more information on local PV in Kubernetes.

Follow the [mount disks](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner/blob/master/docs/operations.md#sharing-a-disk-filesystem-by-multiple-filesystem-pvs) to mount the disk.
