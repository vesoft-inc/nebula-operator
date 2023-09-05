## nebula-exporter

We provide a field `exporter` in CRD to define nebula exporter settings for prometheus.
The exporter field is an object.

Here is the configuration file for NebulaCluster which have an exporter field:
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  exporter:
    image: vesoft/nebula-stats-exporter
    replicas: 1
    # Maximum number of parallel scrape requests. Use 0 to disable.
    maxRequests: 20
    # The regex to filter metrics
    collectRegex: ""
    # The regex to ignore metrics
    ignoreRegex: ""
```