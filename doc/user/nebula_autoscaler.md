## nebula-autoscaler

The nebula-autoscaler is fully compatible with K8S HPA, and you can use it according to the operating mechanism of HPA.
Currently, the nebula-autoscaler only supports automatic scaling of Graphd.
Please refer to the documentation [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) for more information.

### Deploy metrics-server
Please refer to the repo of [metrics-server](https://github.com/kubernetes-sigs/metrics-server).
```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Modify startup parameters and add flag kubelet-insecure-tls to exempt the authentication between metrics-server and kubelet.
# For testing purposes only.
- --cert-dir=/tmp
- --secure-port=4443
- --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
- --kubelet-use-node-status-port
- --kubelet-insecure-tls
- --metric-resolution=15s
```

### Test metrics API
```shell
$ kubectl get --raw "/apis/metrics.k8s.io/v1beta1/namespaces/default/pods/nebula-graphd-1" | jq '.'
{
  "kind": "PodMetrics",
  "apiVersion": "metrics.k8s.io/v1beta1",
  "metadata": {
    "name": "nebula-graphd-1",
    "namespace": "default",
    "creationTimestamp": "2023-09-27T13:39:54Z",
    "labels": {
      "app.kubernetes.io/cluster": "nebula",
      "app.kubernetes.io/component": "graphd",
      "app.kubernetes.io/managed-by": "nebula-operator",
      "app.kubernetes.io/name": "nebula-graph",
      "controller-revision-hash": "nebula-graphd-56cf5f8b66",
      "statefulset.kubernetes.io/pod-name": "nebula-graphd-1"
    }
  },
  "timestamp": "2023-09-27T13:39:48Z",
  "window": "15.015s",
  "containers": [
    {
      "name": "graphd",
      "usage": {
        "cpu": "323307n",
        "memory": "12644Ki"
      }
    }
  ]
}
 
$ kubectl get --raw "/apis/metrics.k8s.io/v1beta1/nodes/192-168-8-35" | jq '.'
{
  "kind": "NodeMetrics",
  "apiVersion": "metrics.k8s.io/v1beta1",
  "metadata": {
    "name": "192-168-8-35",
    "creationTimestamp": "2023-09-27T14:00:13Z",
    "labels": {
      "beta.kubernetes.io/arch": "amd64",
      "beta.kubernetes.io/os": "linux",
      "kubernetes.io/arch": "amd64",
      "kubernetes.io/hostname": "192-168-8-35",
      "kubernetes.io/os": "linux",
      "nebula": "cloud",
      "node-role.kubernetes.io/control-plane": "",
      "node.kubernetes.io/exclude-from-external-load-balancers": ""
    }
  },
  "timestamp": "2023-09-27T14:00:00Z",
  "window": "20.045s",
  "usage": {
    "cpu": "164625163n",
    "memory": "8616740Ki"
  }
}
```

### Autoscaler
Here is the sample without behavior:
```yaml
apiVersion: autoscaling.nebula-graph.io/v1alpha1
kind: NebulaAutoscaler
metadata:
  name: nebula-autoscaler
spec:
  nebulaClusterRef:
    name: nebula
  graphdPolicy:
    minReplicas: 2
    maxReplicas: 5
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
  pollingPeriod: 30s

```

Here is the sample with behavior:
```yaml
apiVersion: autoscaling.nebula-graph.io/v1alpha1
kind: NebulaAutoscaler
metadata:
  name: nebula-autoscaler
spec:
  nebulaClusterRef:
    name: nebula
  graphdPolicy:
    minReplicas: 2
    maxReplicas: 5
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
    behavior:
      scaleDown:
        stabilizationWindowSeconds: 300
        policies:
        - type: Percent
          value: 100
          periodSeconds: 15
      scaleUp:
        stabilizationWindowSeconds: 0
        policies:
        - type: Percent
          value: 100
          periodSeconds: 15
        - type: Pods
          value: 4
          periodSeconds: 15
        selectPolicy: Max
  pollingPeriod: 30s
```

### Verify status
```shell
$ kubectl get nc
NAME     READY   GRAPHD-DESIRED   GRAPHD-READY   METAD-DESIRED   METAD-READY   STORAGED-DESIRED   STORAGED-READY   AGE
nebula   True    2                2              1               1             3                  3                20h
$ kubectl get na
NAME                REFERENCE   MIN-REPLICAS   MAX-REPLICAS   CURRENT-REPLICAS   ACTIVE   ABLETOSCALE   LIMITED   READY   AGE
nebula-autoscaler   nebula      2              5              2                  True     True          True      True    19h
```