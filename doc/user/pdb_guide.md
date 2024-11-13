## Pod Disruption Budget
The Nebula Cluster Helm chart has a feature to automatically start pod disruption budgets (PDBs) for all services it controls (metad, graphd, storaged) if requested by you. PDBs help minimize latency issues by ensuring a minimum number of required pods for the requsted service are available at all times during voluntary node outages. You may specify the minimum pods available or the maximum pods unavailable for a given service and whether to apply the PDB to all nebula graph services or just a particular service.

### Prereqisites
* Nebula operator is installed
* Nebula operator version >= 1.8.2

### Warning
The Nebula Cluster pod disruption budget feature is based on the [kubernetes PDB](https://kubernetes.io/docs/tasks/run-application/configure-pdb/), so it will only be enforced for voluntary disruptions (i.e. draining a node for maintance, rolling update). Any involutary distuptions (i.e. node down, forced update) will cause the PDB to be overridden. There's nothing Nebula can do to prevent this due to the limitations of kubernetes.

### Using the Pod Disruption Budget Feature In the Given Helm Chart

#### Enableing the pdb feature:
The PDBs feature is disabled for all Nebula Cluster services by default. To enable and use this feature for all services, look for the `pdb:` section in the [values.yaml](https://raw.githubusercontent.com/vesoft-inc/nebula-operator/refs/heads/add-documentation/charts/nebula-cluster/values.yaml) file for the given helm chart and set the `enable` field for all three services to `true` like the example below. 
```yaml
pdb:
  graphd:
    enable: true
    minAvailable: 2
    maxUnavailable: -1
  metad:
    enable: true
    minAvailable: 3
    maxUnavailable: -1
  storaged:
    enable: true
    minAvailable: 3
    maxUnavailable: -1
```

To enable this feature for a particular service (i.e. metad), only set the `enable` field for that service to `true` and Leave the other `enable` fields as `false` as shown in the example below. This can be done for any 1 service or a combination of 2 services.
```yaml
# The pdb is only enabled for metad. The enable fields for both graphd and storaged are set to false.
pdb:
  graphd:
    enable: false
    minAvailable: 2
    maxUnavailable: -1
  metad:
    enable: true
    minAvailable: 3
    maxUnavailable: -1
  storaged:
    enable: false
    minAvailable: 3
    maxUnavailable: -1
```

#### Setting the minAvailable and maxUnavailable Fields:
##### 1. minAvailable:
The `minAvailable` field is used to specify that the PDB should enforce a mimimum number of pods required to be running at a given time for a given service. To use the `minAvailable` field for a given service set it to a positive integer and set the `maxUnavailable` field to `-1` as shown in the example below.
```yaml
# The minAvailable field is used for all 3 services as they're all set to positive values
pdb:
  graphd:
    enable: true
    minAvailable: 2
    maxUnavailable: -1
  metad:
    enable: true
    minAvailable: 3
    maxUnavailable: -1
  storaged:
    enable: true
    minAvailable: 3
    maxUnavailable: -1
```

##### 2. maxUnavailable:
The `maxUnavailable` field is used to specify that the PDB should enforce a maximum number of pods that can be down at a given time for a given service. To use the `maxUnavailable` field for a given service set it to a positive integer and set the `minAvailable` field to `-1` as shown in the example below.
```yaml
# The maxUnavailable field is used for all 3 services as they're all set to positive values
pdb:
  graphd:
    enable: true
    minAvailable: -1
    maxUnavailable: 1
  metad:
    enable: true
    minAvailable: -1
    maxUnavailable: 1
  storaged:
    enable: true
    minAvailable: -1
    maxUnavailable: 1
```

##### Notes: 
1. Only 1 of `minAvailable` or `maxUnavailable` may be set to a positive integer at a time for a given service. Setting both fields to positive integers for a given service will result in `minAvailable` being used.
2. An error will result if both `minAvailable` and `maxUnavailable` are set to `-1` for a given service.
3. It's ok to use `minAvailable` for one service and `maxUnavailable` for another.
