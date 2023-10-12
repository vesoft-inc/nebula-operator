# Admission webhook

Admission webhooks are HTTP callbacks that receive admission requests and do something with them. There are two types of admission webhooks, 
validating admission webhook and mutating admission webhook. Mutating admission webhooks are invoked first, and can modify objects sent to the API server
to enforce custom defaults. After all object modifications are complete, and after the incoming object is validated by the API server, 
validating admission webhooks are invoked and can reject requests to enforce custom policies.

The nebula-operator controller-manager starts a built-in admission webhook server and manages policies about how to validate NebulaCluster.

Follow this guide to enable webhook.

### Deploy cert-manager
Refer to the [cert-manager installation](https://cert-manager.io/docs/installation) to get started.

### Enable admission webhook
```yaml
# helm chart nebula-operator values.yaml, set `create` to true
admissionWebhook:
  create: true
  # The TCP port the Webhook server binds to. (default 9443)
  webhookBindPort: 9443
```

Verify resource Issuer and Certificate status
```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  annotations:
    meta.helm.sh/release-name: nebula-operator
    meta.helm.sh/release-namespace: default
  creationTimestamp: "2023-09-29T04:15:20Z"
  generation: 1
  labels:
    app.kubernetes.io/component: admission-webhook
    app.kubernetes.io/instance: nebula-operator
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: nebula-operator
    app.kubernetes.io/version: 1.7.0
    helm.sh/chart: nebula-operator-1.7.0
  name: nebula-operator-webhook-issuer
  namespace: default
  resourceVersion: "109935202"
  uid: 244015eb-2991-4cb9-befc-a35fba0eadce
spec:
  selfSigned: {}
status:
  conditions:
  - lastTransitionTime: "2023-09-29T04:15:20Z"
    observedGeneration: 1
    reason: IsReady
    status: "True"
    type: Ready
 
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  annotations:
    meta.helm.sh/release-name: nebula-operator
    meta.helm.sh/release-namespace: default
  creationTimestamp: "2023-09-29T04:15:20Z"
  generation: 1
  labels:
    app.kubernetes.io/component: admission-webhook
    app.kubernetes.io/instance: nebula-operator
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: nebula-operator
    app.kubernetes.io/version: 1.7.0
    helm.sh/chart: nebula-operator-1.7.0
  name: nebula-operator-webhook-cert
  namespace: default
  resourceVersion: "109935196"
  uid: 7b03e317-354e-4cad-9832-96a74453c462
spec:
  dnsNames:
  - nebula-operator-webhook-service.default.svc
  - nebula-operator-webhook-service.default.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: nebula-operator-webhook-issuer
  secretName: nebula-operator-webhook-secret
status:
  conditions:
  - lastTransitionTime: "2023-09-29T04:15:20Z"
    message: Certificate is up to date and has not expired
    observedGeneration: 1
    reason: Ready
    status: "True"
    type: Ready
  notAfter: "2023-12-19T18:03:06Z"
  notBefore: "2023-09-20T18:03:06Z"
  renewalTime: "2023-11-19T18:03:06Z"
```

### Validate rules
- Append storage volume
```shell
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"storaged": {"dataVolumeClaims":[{"resources": {"requests": {"storage": "2Gi"}}, "storageClassName": "local-path"},{"resources": {"requests": {"storage": "3Gi"}}, "storageClassName": "fask-disks"}]}}}'
Error from server: admission webhook "nebulaclustervalidating.nebula-graph.io" denied the request: spec.storaged.dataVolumeClaims: Forbidden: storaged dataVolumeClaims is immutable
- ```

- Shrink PV
```shell
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"storaged": {"dataVolumeClaims":[{"resources": {"requests": {"storage": "1Gi"}}, "storageClassName": "fast-disks"}]}}}'
Error from server: admission webhook "nebulaclustervalidating.nebula-graph.io" denied the request: spec.storaged.dataVolumeClaims: Invalid value: resource.Quantity{i:resource.int64Amount{value:1073741824, scale:0}, d:resource.infDecAmount{Dec:(*inf.Dec)(nil)}, s:"1Gi", Format:"BinarySI"}: data volume size can only be increased
- ```

- Modify thrift ports
```shell
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"graphd": {"port": 8669}}}'
Error from server: admission webhook "nebulaclustervalidating.nebula-graph.io" denied the request: spec.graphd.port: Invalid value: 8669: field is immutable
- ```

- Intermediate state scaling
```shell
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"storaged": {"replicas": 5}}}'
nebulacluster.apps.nebula-graph.io/nebula patched
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"storaged": {"replicas": 3}}}'
Error from server: admission webhook "nebulaclustervalidating.nebula-graph.io" denied the request: [spec.storaged: Forbidden: field is immutable while in ScaleOut phase, spec.storaged.replicas: Invalid value: 3: field is immutable while not in Running phase]
- ```

- HA mode
```shell
# Create a nebula cluster with 2 graphd, 3 metad, and 3 storaged to meet the minimum HA configuration requirement.
$ kubectl annotate nc nebula nebula-graph.io/ha-mode=true
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"graphd": {"replicas":1}}}'
Error from server: admission webhook "nebulaclustervalidating.nebula-graph.io" denied the request: spec.graphd.replicas: Invalid value: 1: should be at least 2 in HA mode
- ```