## Installing Add-ons

**Caution:**
This section links to third party projects that provide functionality required by nebula-operator. The nebula-operator
project authors aren't responsible for these projects.

## coredns

[CoreDNS](https://coredns.io/) is a flexible, extensible DNS server which can
be [installed](https://github.com/coredns/deployment/tree/master/kubernetes) as the in-cluster DNS for pods.

NebulaGraph each component can communicate via DNS like _x.default.svc.cluster.local_, coredns is used for address
resolution.

## cert-manager

**Note:**
If you set helm chart nebula-operator _.Values.admissionWebhook.create_ to false, the cert-manager is not needed.

[cert-manager](https://cert-manager.io/) is a tool that automates certificate management. It makes use of extending the
Kubernetes API server using a Webhook server to provide dynamic admission control over cert-manager resources.

Refer to the [cert-manager installation documentation](https://cert-manager.io/docs/installation/) to get
started.

cert-manager is used for validating NebulaGraph each component replicas, if you run it in production environment and
care high availability, you should set  _.Values.admissionWebhook.create_ to true and install cert-manager.

## openkruise

**Note:**
If you set helm chart nebula-operator _.Values.enableKruise.create_ to false, the kruise suites is not needed.

[openkruise](https://openkruise.io/)  is a full set of standard extensions for Kubernetes. It works well with original
Kubernetes and provides more powerful and efficient features for managing applications Pods, sidecar containers, and
even images on Node.

Refer to the [openkruise installation documentation](https://openkruise.io/docs/installation) to get started.
