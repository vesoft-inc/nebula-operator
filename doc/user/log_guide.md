### Log rotation

We use the sidecar container to clean NebulaGraph logs and run logs archiving tasks every hour.

```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
spec:
  graphd:
    config:
      # Whether logging files' name contain timestamp.
      "timestamp_in_logfile_name": "false"
  metad:
    config:
      "timestamp_in_logfile_name": "false"
  storaged:
    config:
      "timestamp_in_logfile_name": "false"
  logRotate:
    # Log files are rotated count times before being removed  
    rotate: 5
    # Log files are rotated only if they grow bigger than size bytes
    size: "100M"
```

### Write log to stdout

If you do not need to mount additional log disks in order to save costs on the cloud,
and collect them through services such as fluent-bit and send them to the log center, you can refer to the configuration below.

```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
spec:
  graphd:
    config:
      # Whether to redirect stdout and stderr to separate output files
      redirect_stdout: "false"
      # The numbers of severity level INFO, WARNING, ERROR, and FATAL are 0, 1, 2, and 3, respectively.
      stderrthreshold: "0"
      # Logs are written to standard error instead of to files
      logtostderr: "true"
    image: vesoft/nebula-graphd
    replicas: 1
    resources:
      requests:
        cpu: 500m
        memory: 500Mi
    service:
      externalTrafficPolicy: Local
      type: NodePort
    version: v3.5.0
  imagePullPolicy: Always
  metad:
    config:
      redirect_stdout: "false"
      stderrthreshold: "0"
      logtostderr: "true"
    dataVolumeClaim:
      resources:
        requests:
          storage: 1Gi
      storageClassName: ebs-sc
    image: vesoft/nebula-metad
    replicas: 1
    resources:
      requests:
        cpu: 500m
        memory: 500Mi
    version: v3.5.0
  reference:
    name: statefulsets.apps
    version: v1
  schedulerName: default-scheduler
  storaged:
    config:
      redirect_stdout: "false"
      stderrthreshold: "0"
      logtostderr: "true"
    dataVolumeClaims:
    - resources:
        requests:
          storage: 1Gi
      storageClassName: ebs-sc
    enableAutoBalance: true
    enableForceUpdate: false
    image: vesoft/nebula-storaged
    replicas: 1
    resources:
      requests:
        cpu: 500m
        memory: 500Mi
    version: v3.5.0
  unsatisfiableAction: ScheduleAnyway
```
