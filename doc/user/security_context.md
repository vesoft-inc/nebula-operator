## SecurityContext

We provide a field `securityContext` in CRD to define privilege and access control settings for nebula Container.
The securityContext field is a [SecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) object. 
Security settings that you specify for a nebula Container will not affect other containers.

Here is the configuration file for Graphd which have a securityContext field:
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  graphd:
    securityContext:
      capabilities:
        add: ["SYS_ADMIN"]
    resources:
      requests:
        cpu: "200m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    logVolumeClaim:
      resources:
        requests:
          storage: 1Gi
      storageClassName: local-path
    replicas: 1
    image: vesoft/nebula-graphd
    version: v3.5.0
```
