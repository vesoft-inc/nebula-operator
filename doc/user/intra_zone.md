## Intra-zone

AWS provides the capability to control the availability zone (AZ) placement of resources, 
which can help user optimize costs and improve performance by keeping related components of application within the same AZ. 
This is particularly useful in scenarios where user want to minimize data transfer costs and latency.

NebulaGraph allows the client to send requests only to the Graphd in the same AZ, 
and the Graphd to send requests only to the Storaged in the same AZ, achieving the goal of cost savings.

Here is the configuration file for NebulaCluster to enable this feature:
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  # Alpine image built with linux tools.
  alpineImage: "reg.vesoft-inc.com/nebula-alpine:latest"
  graphd:
    config:
      # Prioritize to send queries to storaged in the same zone
      prioritize_intra_zone_reading: "true"
      # Stick to intra zone routing if unable to
      # find the storaged hosting the requested part in the same zone
      stick_to_intra_zone_on_failure: "true"
    resources:
      requests:
        cpu: "500m"
        memory: "500Mi"
      limits:
        cpu: "2"
        memory: "2Gi"
    logVolumeClaim:
      resources:
        requests:
          storage: 1Gi
      storageClassName: local-path
    replicas: 1
    image: reg.vesoft-inc.com/nebula-graphd-ent
    version: v3.6.0
  metad:
    config:
      # A list of zones (e.g., AZ) split by comma
      zone_list: az1,az2,az3
    licenseManagerURL: "192.168.8.53:9119"
    resources:
      requests:
        cpu: "500m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: reg.vesoft-inc.com/nebula-metad-ent
    version: v3.6.0
    dataVolumeClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: local-path
    logVolumeClaim:
      resources:
        requests:
          storage: 1Gi
      storageClassName: local-path
```