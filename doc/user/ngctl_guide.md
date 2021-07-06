
# Overview
ngctl is a terminal cmd tool for nebula-operator, it has the following commands:
- [ngctl use](#ngctl-use)
- [ngctl console](#ngctl-console)
- [ngctl list](#ngctl-list)
- [ngctl check](#ngctl-check)
- [ngctl info](#ngctl-info)

## ngctl use
`ngctl use` specify a nebula cluster to use. By using a certain cluster, you may omit --nebulacluster option in many control commands.

```
Examples:
  # specify a nebula cluster to use
  ngctl use demo-cluster

  # specify kubernetes context and namespace
  ngctl use demo-cluster -n test-system
```
## ngctl console
`ngctl console` create a nebula-console pod and connect to the specified nebula cluster.

```
Examples:
  # open console to the current nebula cluster, which is set by 'ngctl use' command
  ngctl console
  
  # Open console to the specified nebula cluster
  ngctl console --nebulacluster=nebula
```
## ngctl list
`ngctl list` list nebula clusters or there sub resources. Its usage is the same as `kubectl get`, but only resources related to nbuela cluster are displayed.
```
Examples:
  # List all nebula clusters.
  ngctl list
  
  # List all nebula clusters in all namespaces.
  ngctl list -A
  
  # List all nebula clusters with json format.
  ngctl list -o json
  
  # List nebula cluster sub resources with specified cluster name.
  ngctl list pod --nebulacluster=nebula
  
  # You can also use 'use' command to specify a nebula cluster.
  use nebula
  ngctl list pod
  
  # Return only the metad's phase value of the specified pod.
  ngctl list -o template --template="{{.status.graphd.phase}}" NAME
  
  # List image information in custom columns.
  ngctl list -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,IMAGE:.spec.graphd.image
```

## ngctl check
`ngctl check` check whether the specified nebula cluster resources are ready. Command will print the error message from nebula cluster resource conditions, help you locate the reason quickly.

```
Examples:
  # check whether the specified nebula cluster is ready
  ngctl check
  
  # check specified nebula cluster pods
  ngctl check pods --nebulacluster=nebula
```

## ngctl info
`ngctl info` get current nebula cluster information, the cluster is set by 'ngctl use' command or use `--nebulacluster` flag.

```Examples:
  # get current nebula cluster information, which is set by 'ngctl use' command
  ngctl info

  # get current nebula cluster information, which is set by '--nebulacluster' flag
  ngctl info --nebulacluster=nebula
```
## ngctl version
`nfgctl version` print the cli and nebula operator version

```bash
Examples:
  # Print the cli and nebula operator version
  ngctl version
```

