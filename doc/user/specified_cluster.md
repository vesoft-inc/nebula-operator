## Specified cluster

You can restrict the management scope of the Nebula cluster by specifying the namespace or cluster selector. 
This feature is applicable to scenarios such as operator grayscale release or management cluster splitting.

You can specify the cluster through the flags of the controller-manager.
```shell
--nebula-object-selector string     nebula object selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2).
 
--watch-namespaces strings    Namespaces restricts the controller watches for updates to Kubernetes objects. If empty, all namespaces are watched. Multiple namespaces seperated by comma.(e.g. ns1,ns2,ns3).
```

Configuration steps:
Modify the parameters watchNamespaces or nebulaObjectSelector when deploying operator by helm chart.
```yaml
# Namespaces restricts the controller-manager watches for updates to Kubernetes objects. If empty, all namespaces are watched.
# e.g. ns1,ns2,ns3
watchNamespaces: ""
 
# nebula object selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2).
nebulaObjectSelector: ""
```