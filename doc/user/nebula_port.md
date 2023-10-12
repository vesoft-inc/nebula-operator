### Nebula port configurable

We provide the fields `port` and `httpPort` in CRD to define the port settings for each component in NebulaGraph.
- The Thrift port can be configured upon creation, but changes are prohibited when the cluster is running.
- The HTTP port can be configured at any time.

Here is the configuration file for NebulaCluster which have a custom HTTP port:
```yaml
apiVersion: apps.nebula-graph.io/v1alpha1
kind: NebulaCluster
metadata:
  name: nebula
  namespace: default
spec:
  graphd:
    port: 9669
    httpPort: 8080
    config:
      logtostderr: "true"
      redirect_stdout: "false"
      stderrthreshold: "0"
    resources:
      requests:
        cpu: "200m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: vesoft/nebula-graphd
    version: v3.6.0
  metad:
    port: 9559
    httpPort: 8081
    config:
      redirect_stdout: "false"
      stderrthreshold: "0"
      logtostder: "true"
    resources:
      requests:
        cpu: "300m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: vesoft/nebula-metad
    version: v3.6.0
    dataVolumeClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: local-path
  storaged:
    port: 9779
    httpPort: 8082
    config:
      redirect_stdout: "false"
      stderrthreshold: "0"
      logtostder: "true"
    resources:
      requests:
        cpu: "300m"
        memory: "500Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 1
    image: vesoft/nebula-storaged
    version: v3.6.0
    dataVolumeClaims:
    - resources:
        requests:
          storage: 2Gi
      storageClassName: local-path
    enableAutoBalance: true
  reference:
    name: statefulsets.apps
    version: v1
  schedulerName: default-scheduler
  imagePullPolicy: IfNotPresent
  imagePullSecrets:
  - name: nebula-image
  enablePVReclaim: true
  topologySpreadConstraints:
  - topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: "ScheduleAnyway"
```

Verify the configuration:
```shell
$ kubectl get svc
NAME                        TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                      AGE
nebula-graphd-headless      ClusterIP   None             <none>        9669/TCP,8080/TCP            10m
nebula-graphd-svc           ClusterIP   10.102.13.115    <none>        9669/TCP,8080/TCP            10m
nebula-metad-headless       ClusterIP   None             <none>        9559/TCP,8081/TCP            11m
nebula-storaged-headless    ClusterIP   None             <none>        9779/TCP,8082/TCP,9778/TCP   11m
$ curl 10.102.13.115:8080/status
{"git_info_sha":"537f942","status":"running"}
```