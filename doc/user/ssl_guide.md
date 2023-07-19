### SSL encryption

NebulaGraph supports data transmission with SSL encryption between clients, the Graph service,
the Meta service, and the Storage service. This topic describes how to enable SSL encryption.

NebulaGraph supports three encryption policies:

- Encrypt the data transmission between clients, the Graph service, the Meta service, and the Storage service.   
  To enable this policy, add `enable_ssl = true` to each component.

  ```yaml
  apiVersion: apps.nebula-graph.io/v1alpha1
  kind: NebulaCluster
  metadata:
    name: nebula
    namespace: default
  spec:
    sslCerts:
      serverSecret: "server-cert"
      clientSecret: "client-cert"
      caSecret: "ca-cert"
    graphd:
      config:
        enable_ssl: "true"
    metad:
      config:
        enable_ssl: "true"
    storaged:
      config:
        enable_ssl: "true"
  ```

- Encrypt the data transmission between clients and the Graph service.  
  This policy is applicable when the clusters are set up in the same server room. Only the port of the Graph service is
  open to the outside,
  as other services can communicate over the internal network without encryption.
  To enable this policy, add `enable_graph_ssl = true` to the graphd component.

  ```yaml
  apiVersion: apps.nebula-graph.io/v1alpha1
  kind: NebulaCluster
  metadata:
    name: nebula
    namespace: default
  spec:
    sslCerts:
      serverSecret: "server-cert"
      caSecret: "ca-cert"
    graphd:
      config:
        enable_graph_ssl: "true"
  ```

- Encrypt the data transmission related to the Meta service in the cluster.   
  This policy applies to transporting classified information to the Meta service.
  To enable this policy, add `enable_meta_ssl = true` to each component.

  ```yaml
  apiVersion: apps.nebula-graph.io/v1alpha1
  kind: NebulaCluster
  metadata:
    name: nebula
    namespace: default
  spec:
    sslCerts:
      serverSecret: "server-cert"
      clientSecret: "client-cert"
      caSecret: "ca-cert"
    graphd:
      config:
        enable_meta_ssl: "true"
    metad:
      config:
        enable_meta_ssl: "true"
    storaged:
      config:
        enable_meta_ssl: "true"
  ```

The full configuration of sslCerts:
```yaml
sslCerts:
  # Name of the server cert secret
  serverSecret: "server-cert"
  # The key to server PEM encoded public key certificate, default name is tls.crt
  serverCert: ""
  # The key to server private key associated with given certificate, default name is tls.key
  serverKey: ""
  # Name of the client cert secret
  clientSecret: "client-cert"
  # The key to server PEM encoded public key certificate, default name is tls.crt
  clientCert: ""
  # The key to client private key associated with given certificate, default name is tls.key
  clientKey: ""
  # Name of the CA cert secret
  caSecret: "ca-cert"
  # The key to CA PEM encoded public key certificate, default name is ca.crt
  caCert: ""
  # InsecureSkipVerify controls whether a client verifies the server's certificate chain and host name
  insecureSkipVerify: false
```