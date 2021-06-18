# nebula client service

For every nebula cluster created, the nebula operator will create a graphd service in the same namespace with the name `<cluster-name>-graphd-svc`.

```shell script
$ kubectl get service -l app.kubernetes.io/cluster=nebula
NAME                       TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                          AGE
nebula-graphd-svc          ClusterIP   10.98.213.34   <none>        9669/TCP,19669/TCP,19670/TCP                     23h
nebula-metad-headless      ClusterIP   None           <none>        9559/TCP,19559/TCP,19560/TCP                     23h
nebula-storaged-headless   ClusterIP   None           <none>        9779/TCP,19779/TCP,19780/TCP,9778/TCP            23h
```

The client service is of type `ClusterIP` and accessible only from within the Kubernetes CNI network.

For example, access the service from a pod in the cluster:

```shell script
$ kubectl run --rm -ti --image vesoft/nebula-console:v2 --restart=Never -- /bin/sh
/ # nebula-console -u user -p password --address=nebula-graphd-svc --port=9669
2021/04/12 08:16:30 [INFO] connection pool is initialized successfully

Welcome to Nebula Graph!
(user@nebula) [(none)]> 
```

If accessing this service from a different namespace than that of the nebula cluster, use the fully qualified domain name (FQDN) `http://<cluster-name>-graphd.<cluster-namespace>.svc.<CLUSTER_DOMAIN>:9669`.

Generally, the default value of `CLUSTER_DOMAIN` is `cluster.local`.

## Accessing the service from outside the cluster

To access the graphd service of the nebula cluster from outside the Kubernetes cluster, expose a new client service of type `NodePort`. If using a cloud provider like Azure, GCP or AWS, setting the type to `LoadBalancer` will automatically create the load balancer with a publicly accessible IP.

The spec for this service will use the label selector `app.kubernetes.io/cluster: <cluster-name>` `app.kubernetes.io/component: graphd` to load balance the client requests over the graphd pods in the cluster.

For example, create a nodePort service for the cluster described above:

```shell script
$ cat config/samples/graphd-nodeport-service.yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/cluster: nebula
    app.kubernetes.io/component: graphd
    app.kubernetes.io/managed-by: nebula-operator
    app.kubernetes.io/name: nebula-graph
  name: nebula-graphd-svc-nodeport
  namespace: default
spec:
  externalTrafficPolicy: Local
  ports:
  - name: thrift
    port: 9669
    protocol: TCP
    targetPort: 9669
  - name: http
    port: 19669
    protocol: TCP
    targetPort: 19669
  selector:
    app.kubernetes.io/cluster: nebula
    app.kubernetes.io/component: graphd
    app.kubernetes.io/managed-by: nebula-operator
    app.kubernetes.io/name: nebula-graph
  type: NodePort

$ kubectl create -f graphd-nodeport-service.yaml
```

```shell script
$ kubectl get services
NAME                           TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                          AGE
nebula-graphd-svc              ClusterIP   10.98.213.34   <none>        9669/TCP,19669/TCP,19670/TCP                     23h
nebula-graphd-svc-nodeport     NodePort    10.107.153.129 <none>        9669:32236/TCP,19669:31674/TCP,19670:31057/TCP   24h
nebula-metad-headless          ClusterIP   None           <none>        9559/TCP,19559/TCP,19560/TCP                     23h
nebula-storaged-headless       ClusterIP   None           <none>        9779/TCP,19779/TCP,19780/TCP,9778/TCP            23h
```

The graphd client API should now be accessible from outside the Kubernetes cluster:

```shell script
/ # nebula-console -u user -p password --address=192.168.8.26 --port=9669
2021/04/12 08:50:32 [INFO] connection pool is initialized successfully

Welcome to Nebula Graph!
(user@nebula) [(none)]> 
```
