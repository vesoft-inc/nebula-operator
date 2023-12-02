## nebula-console

We provide a field `console` in CRD to define nebula console settings for NebulaGraph.
The console field is an object.

Here is the configuration file for NebulaCluster which have a console field:
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  console:
    image: vesoft/nebula-console
    version: nightly
    username: "demo"
    password: "test"
```

If you enable `enable_ssl` or `enable_graph_ssl` to true in the `config` field, nebula-console will enable SSL when connecting to Graphd.
At the same time, you need to configure CA and Client certificates within sslCerts.

```yaml
spec:
  sslCerts:
    caCert: root.crt
    caSecret: ca-cert
    clientCert: tls.crt
    clientKey: tls.key
    clientSecret: client-cert
```

Here is the output of a Running state pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/cluster: nebula
    app.kubernetes.io/component: console
    app.kubernetes.io/managed-by: nebula-operator
    app.kubernetes.io/name: nebula-graph
  name: nebula-console
  namespace: default
  ownerReferences:
  - apiVersion: apps.nebula-graph.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: NebulaCluster
    name: nebula
    uid: af183364-4fc1-4183-9e0a-e5c552a2df33
spec:
  containers:
  - command:
    - nebula-console
    - -addr
    - nebula-graphd-svc
    - -port
    - "9669"
    - -u
    - root
    - -p
    - nebula
    - -enable_ssl
    - -ssl_cert_path
    - /tmp/client.crt
    - -ssl_private_key_path
    - /tmp/client.key
    - -ssl_root_ca_path
    - /tmp/ca.crt
    image: vesoft/nebula-console:nightly
    imagePullPolicy: Always
    name: console
    stdin: true
    stdinOnce: true
    tty: true
    volumeMounts:
    - mountPath: /tmp/client.crt
      name: client-crt
      readOnly: true
      subPath: client.crt
    - mountPath: /tmp/client.key
      name: client-key
      readOnly: true
      subPath: client.key
    - mountPath: /tmp/ca.crt
      name: client-ca-crt
      readOnly: true
      subPath: ca.crt
  schedulerName: default-scheduler
  serviceAccountName: nebula-sa
  volumes:
  - name: client-crt
    secret:
      items:
      - key: tls.crt
        path: client.crt
      secretName: client-cert
  - name: client-key
    secret:
      items:
      - key: tls.key
        path: client.key
      secretName: client-cert
  - name: client-ca-crt
    secret:
      items:
      - key: root.crt
        path: ca.crt
      secretName: ca-cert
```