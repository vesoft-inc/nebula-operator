## Suspend cluster

After suspend the cluster, cluster configuration and storage volumes are reserved, resources occupied by the nebula cluster
will be released.

```shell
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"suspend":true}}'
nebulacluster.apps.nebula-graph.io/nebula patched

# Check the status
$ kubectl get nc nebula 
NAME     READY   GRAPHD-DESIRED   GRAPHD-READY   METAD-DESIRED   METAD-READY   STORAGED-DESIRED   STORAGED-READY   AGE
nebula   False   1                               1                             9                                   15h

# phase is Suspend
status:
  conditions:
  - lastTransitionTime: "2023-11-12T06:54:13Z"
    lastUpdateTime: "2023-11-12T06:54:13Z"
    message: Workload is in progress
    reason: WorkloadNotUpToDate
    status: "False"
    type: Ready
  graphd:
    phase: Suspend
    version: v3.6.0
  metad:
    phase: Suspend
    version: v3.6.0
  observedGeneration: 8
  storaged:
    balancedAfterFailover: true
    phase: Suspend
    version: v3.6.0
  version: 3.6.0

-------------------
# Recovery cluster
$ kubectl patch nc nebula  --type='merge' --patch '{"spec": {"suspend":null}}'
nebulacluster.apps.nebula-graph.io/nebula patched

# Watch the status
$ kubectl get nc nebula -w
NAME     READY   GRAPHD-DESIRED   GRAPHD-READY   METAD-DESIRED   METAD-READY   STORAGED-DESIRED   STORAGED-READY   AGE
nebula   False   1                               1                             9                                   16h
nebula   False   1                               1                             9                                   16h
nebula   False   1                               1               1             9                                   16h
nebula   False   1                               1               1             9                                   16h
nebula   False   1                               1               1             9                                   16h
nebula   False   1                               1               1             9                                   16h
nebula   False   1                               1               1             9                  3                16h
nebula   False   1                               1               1             9                  4                16h

nebula   False   1                               1               1             9                  5                16h
nebula   False   1                               1               1             9                  6                16h
nebula   False   1                               1               1             9                  8                16h
nebula   False   1                1              1               1             9                  8                16h
nebula   False   1                1              1               1             9                  8                16h
nebula   False   1                1              1               1             9                  8                16h
nebula   False   1                               1               1             9                  8                16h
nebula   False   1                               1               1             9                  8                16h
nebula   False   1                               1               1             9                  8                16h
nebula   False   1                               1               1             9                  8                16h
nebula   False   1                               1               1             9                  9                16h
nebula   True    1                1              1               1             9                  9                16h
```
