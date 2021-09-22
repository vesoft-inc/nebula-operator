# configure custom parameter

For each component has a configuration entry, it defines in crd as config which is a map structure, it will be loaded by configamap.
```go
// Config defines a graphd configuration load into ConfigMap
Config map[string]string `json:"config,omitempty"`
```

The below will show you how to configure it.
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  graphd:
    resources:
      requests:
        cpu: "500m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: vesoft/nebula-graphd
    version: v2.5.0
    storageClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: gp2
    config:
      "enable_authorize": "true"
      "auth_type": "password"
...
```

After the custom parameters _enable_authorize_ and _auth_type_ configured, the configmap will overwrite.