# Nebula Operator

Nebula Operator manages [NebulaGraph](https://github.com/vesoft-inc/nebula) clusters on [Kubernetes](https://kubernetes.io) and automates tasks related to operating a NebulaGraph cluster.
It evolved from [NebulaGraph Cloud Service](https://www.nebula-cloud.io/), makes NebulaGraph a truly cloud-native database.

## Quick Start
- [Install Nebula Operator](#install-nebula-operator)
- [Create and Destroy](#create-and-destroy-a-nebula-cluster)
- [Resize](#resize-a-nebula-cluster)
- [Rolling Upgrade](#upgrade-a-nebula-cluster)
- [Failover](#failover)

### install nebula operator
See [install/uninstall nebula operator](doc/user/install_guide.md) .

### Create and destroy a nebula cluster
```bash
$ kubectl create -f config/samples/apps_v1alpha1_nebulacluster.yaml
```
A none ha-mode nebula cluster will be created.
```bash
$ kubectl get pods -l app.kubernetes.io/cluster=nebula
NAME                READY   STATUS    RESTARTS   AGE
nebula-graphd-0     1/1     Running   0          1m
nebula-metad-0      1/1     Running   0          1m
nebula-storaged-0   1/1     Running   0          1m
nebula-storaged-1   1/1     Running   0          1m
nebula-storaged-2   1/1     Running   0          1m
```
See [client service](doc/user/client_service.md) for how to access nebula clusters created by the operator.  
If you are working with [kubeadm locally](https://kubernetes.io/docs/reference/setup-tools/kubeadm/), create a nodePort service and test that nebula is responding:
```shell script
$ kubectl create -f config/samples/graphd-nodeport-service.yaml

# nebula-console -u user -p password --address=192.168.8.26 --port=32236
2021/04/15 16:50:23 [INFO] connection pool is initialized successfully

Welcome to NebulaGraph!
(user@nebula) [(none)]> 
```

Destroy the nebula cluster:  
```bash
$ kubectl delete -f config/samples/apps_v1alpha1_nebulacluster.yaml
```

### Resize a nebula cluster
Create a nebula cluster:

```bash
$ kubectl create -f config/samples/apps_v1alpha1_nebulacluster.yaml
```

In `config/samples/apps_v1alpha1_nebulacluster.yaml` the initial storaged replicas is 3.  
Modify the file and change `replicas` from 3 to 5.
```yaml
  storaged:
    resources:
      requests:
        cpu: "1"
        memory: "1Gi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 5
    image: vesoft/nebula-storaged
    version: v3.1.0
    storageClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: fast-disks
```

Apply the replicas change to the cluster CR:
```
$ kubectl apply -f config/samples/apps_v1alpha1_nebulacluster.yaml
```

The storaged cluster will scale to 5 members (5 pods):
```bash
$ kubectl get pods -l app.kubernetes.io/cluster=nebula
NAME                READY   STATUS    RESTARTS   AGE
nebula-graphd-0     1/1     Running   0          2m
nebula-metad-0      1/1     Running   0          2m
nebula-storaged-0   1/1     Running   0          2m
nebula-storaged-1   1/1     Running   0          2m
nebula-storaged-2   1/1     Running   0          2m
nebula-storaged-3   1/1     Running   0          5m
nebula-storaged-4   1/1     Running   0          5m
```

Similarly we can decrease the size of the cluster from 5 back to 3 by changing the replicas field again and reapplying the change.
```yaml
  storaged:
    resources:
      requests:
        cpu: "1"
        memory: "1Gi"
      limits:
        cpu: "1"
        memory: "1Gi"
    replicas: 3
    image: vesoft/nebula-storaged
    version: v3.1.0
    storageClaim:
      resources:
        requests:
          storage: 2Gi
      storageClassName: fast-disks
```

We should see that storaged cluster will eventually reduce to 3 pods:

```bash
$ kubectl get pods -l app.kubernetes.io/cluster=nebula
NAME                READY   STATUS    RESTARTS   AGE
nebula-graphd-0     1/1     Running   0          10m
nebula-metad-0      1/1     Running   0          10m
nebula-storaged-0   1/1     Running   0          10m
nebula-storaged-1   1/1     Running   0          10m
nebula-storaged-2   1/1     Running   0          10m
```

In addition, you can [Install Nebula Cluster with helm](doc/user/nebula_cluster_helm_guide.md).

### Upgrade a nebula cluster
Create a nebula cluster with the version specified (v3.0.0):

```bash
$ kubectl apply -f config/samples/apps_v1alpha1_nebulacluster.yaml
$ kubectl get pods -l app.kubernetes.io/cluster=nebula
NAME                READY   STATUS    RESTARTS   AGE
nebula-graphd-0     1/1     Running   0          25m
nebula-metad-0      1/1     Running   0          26m
nebula-storaged-0   1/1     Running   0          22m
nebula-storaged-1   1/1     Running   0          24m
nebula-storaged-2   1/1     Running   0          25m
```

The container image version should be v3.0.0:

```
$ kubectl get pods -l app.kubernetes.io/cluster=nebula  -o jsonpath="{.items[*].spec.containers[*].image}" |tr -s '[[:space:]]' '\n' |sort |uniq -c
      1 vesoft/nebula-graphd:v3.0.0
      1 vesoft/nebula-metad:v3.0.0
      3 vesoft/nebula-storaged:v3.0.0
```

Now modify the file `apps_v1alpha1_nebulacluster.yaml` and change the `version` from v3.0.0 to v3.1.0:

Apply the version change to the cluster CR:

```
$ kubectl apply -f config/samples/apps_v1alpha1_nebulacluster.yaml
```

Wait 2 minutes. The container image version should be updated to v3.1.0:

```
$ kubectl get pods -l app.kubernetes.io/cluster=nebula  -o jsonpath="{.items[*].spec.containers[*].image}" |tr -s '[[:space:]]' '\n' |sort |uniq -c
      1 vesoft/nebula-graphd:v3.1.0
      1 vesoft/nebula-metad:v3.1.0
      3 vesoft/nebula-storaged:v3.1.0
```

### Failover
If the minority of nebula components crash, the nebula operator will automatically recover the failure. Let's walk through this in the following steps.  

Create a nebula cluster:
```bash
$ kubectl create -f config/samples/apps_v1alpha1_nebulacluster.yaml
```

Wait until pods are up. Simulate a member failure by deleting a storaged pod:

```bash
$ kubectl delete pod nebula-storaged-2 --now
```

The nebula operator will recover the failure by creating a new pod `nebula-storaged-2`:

```bash
$ kubectl get pods -l app.kubernetes.io/cluster=nebula
NAME                READY   STATUS    RESTARTS   AGE
nebula-graphd-0     1/1     Running   0          15m
nebula-metad-0      1/1     Running   0          15m
nebula-storaged-0   1/1     Running   0          15m
nebula-storaged-1   1/1     Running   0          15m
nebula-storaged-2   1/1     Running   0          19s
```

## Compatibility matrix

Nebula Operator <-> NebulaGraph

|                        | NebulaGraph v2.5 | NebulaGraph v2.6 | NebulaGraph v3.0 | NebulaGraph v3.1 |
|----------------------- |------------------|------------------|------------------| ------------------|
| `v0.8.0`               | ✓                | -                | -                | -                |
| `v0.9.0`*              | ✓                | ✓                | -                | -                |
| `v1.0.0`*              | -                | -                | ✓                | ✓                |
| `v1.1.0`               | -                | -                | ✓                | ✓                |

Key:

* `✓` Compatible.
* `-` Not Compatible.
* `*` Please notice that the StorageClaim is split into LogVolumeClaim and DataVolumeClaim in crd. v0.9.0 can't forward compatible.
* v1.0.0 does not support storage auto scaling.
* v1.x does not support upgrading from v2.x to v3.x.

## FAQ

Please refer to [FAQ.md](FAQ.md)

## Community
Feel free to reach out if you have any questions. The maintainers of this project are reachable via:

- [Filing an issue](https://github.com/vesoft-inc/nebula-operator/issues) against this repo

## Contributing

Contributions are welcome and greatly appreciated. 
- Start by some issues
- Submit Pull Requests to us. Please refer to [how-to-contribute](https://docs.nebula-graph.io/manual-EN/4.contributions/how-to-contribute/).

## Acknowledgements

nebula-operator refers to [tidb-operator](https://github.com/pingcap/tidb-operator). They have made a very good product. We have a similar architecture, although the product pattern is different from the application scenario, we would like to express our gratitude here.

## License

NebulaGraph is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
