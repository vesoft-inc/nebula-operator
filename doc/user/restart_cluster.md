## Restart cluster

The essence of Pod restarting is to reboot the service process. In daily maintenance, for various reasons, it may be necessary 
to restart a certain service of the Nebula cluster, such as abnormal Pod status or the execution of forced restart logic.
Nebula-operator provides the ability to elegantly roll-restart all Pods within a certain service of the cluster, or elegantly
restart a single Storage Pod.

Cluster restart is triggered through Annotation, providing two operations:
- Restart all Pods of the component, `nebula-graph.io/restart-timestamp`
- Restart the specified Pod of the component, `nebula-graph.io/restart-ordinal`, Storage supports graceful restart, 
Meta and Graph service can directly delete Pod.

```shell
# Get timestamp
$ date -u +%s
1699770225

# Triggers a rolling restart of the component
$ kubectl annotate sts nebula-graphd nebula-graph.io/restart-timestamp="1699770225" --overwrite 
statefulset.apps/nebula-graphd annotate

# Watch restart event
$ kubectl get pods -l app.kubernetes.io/cluster=nebula,app.kubernetes.io/component=graphd -w
NAME              READY   STATUS    RESTARTS   AGE
nebula-graphd-0   1/1     Running   0          4m24s


------------------
# Triggers storaged-1 restart
$ kubectl annotate sts nebula-storaged nebula-graph.io/restart-ordinal="1"
statefulset.apps/nebula-storaged annotate

# Watch logs
I1112 14:39:37.934046 3170806 storage_client.go:86] start transfer leader spaceID 5 partitionID 45
I1112 14:39:42.938781 3170806 storaged_updater.go:389] storaged cluster [default/nebula-storaged] transfer leader spaceID 5 partitionID 45 to host nebula-storaged-3.nebula-storaged-headless.default.svc.cluster.local successfully
I1112 14:39:42.938839 3170806 storage_client.go:86] start transfer leader spaceID 5 partitionID 42
I1112 14:39:47.941335 3170806 storaged_updater.go:389] storaged cluster [default/nebula-storaged] transfer leader spaceID 5 partitionID 42 to host nebula-storaged-0.nebula-storaged-headless.default.svc.cluster.local successfully
I1112 14:39:47.941390 3170806 storage_client.go:86] start transfer leader spaceID 5 partitionID 40
I1112 14:39:52.943926 3170806 storaged_updater.go:389] storaged cluster [default/nebula-storaged] transfer leader spaceID 5 partitionID 40 to host nebula-storaged-0.nebula-storaged-headless.default.svc.cluster.local successfully
I1112 14:39:52.943971 3170806 storage_client.go:86] start transfer leader spaceID 5 partitionID 34
I1112 14:39:57.946534 3170806 storaged_updater.go:389] storaged cluster [default/nebula-storaged] transfer leader spaceID 5 partitionID 34 to host nebula-storaged-4.nebula-storaged-headless.default.svc.cluster.local successfully
I1112 14:39:57.946576 3170806 storage_client.go:86] start transfer leader spaceID 5 partitionID 31
I1112 14:40:02.949196 3170806 storaged_updater.go:389] storaged cluster [default/nebula-storaged] transfer leader spaceID 5 partitionID 31 to host nebula-storaged-0.nebula-storaged-headless.default.svc.cluster.local successfully

# Watch restart event
$ kubectl get pods -l app.kubernetes.io/cluster=nebula,app.kubernetes.io/component=storaged -w
NAME                READY   STATUS    RESTARTS   AGE
nebula-storaged-0   1/1     Running   0          13h
nebula-storaged-1   1/1     Running   0          13h
nebula-storaged-2   1/1     Running   0          13h
nebula-storaged-3   1/1     Running   0          13h
nebula-storaged-4   1/1     Running   0          13h
nebula-storaged-5   1/1     Running   0          12h
nebula-storaged-6   1/1     Running   0          12h
nebula-storaged-7   1/1     Running   0          12h
nebula-storaged-8   1/1     Running   0          12h


nebula-storaged-1   1/1     Running       0          13h
nebula-storaged-1   1/1     Terminating   0          13h
nebula-storaged-1   0/1     Terminating   0          13h
nebula-storaged-1   0/1     Terminating   0          13h
nebula-storaged-1   0/1     Terminating   0          13h
nebula-storaged-1   0/1     Terminating   0          13h
nebula-storaged-1   0/1     Pending       0          0s
nebula-storaged-1   0/1     Pending       0          0s
nebula-storaged-1   0/1     ContainerCreating   0          0s
nebula-storaged-1   0/1     Running             0          1s
nebula-storaged-1   1/1     Running             0          10s



```