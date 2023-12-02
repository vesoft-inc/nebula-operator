## local PV failover

Compared to network block storage, local storage is physically mounted to the virtual machine instance, providing superior 
I/O operations per second (IOPS), and very low read-write latency due to this tight coupling. While using local storage
can enhance performance, it also requires certain trade-offs in terms of service availability, data persistence, and flexibility.
Because of these factors, local storage does not automatically replicate, so all data on local storage may be lost if the
virtual machine stops or is terminated for any reason. Nebula-graph's storage service itself has data redundancy capabilities,
storing three replicas for each partition. If a node fails, the partition associated with this node will re-elect a leader,
and the read-write operations will automatically shift to the healthy node. When using network block storage, if a machine fails,
the Pod can be rescheduled to a new machine and mount the original storage volume. But for local storage, due to node affinity
constraints, the Pod can only be in a Pending state affecting the overall availability of the cluster without unbinding the storage volume.

### Solution
- nebula cluster status check is divided into two parts: the status of nebula service and k8s Node status. The nebula service will periodically
report heartbeat to the Meta service, so you can check the status of each service in the current cluster through the interface 
exposed by the Meta service.
- When the host status of the Storage service is found to be "OFFLINE", update the host with the status of "OFFLINE" 
to the status field of the resource object NebulaCluster for later fault transfer.
- In order to prevent misjudgment of the check, it is allowed to set the tolerance OFFLINE time, and the fault transfer 
process will be executed after exceeding this period.

The controller will perform the following operations while executing automatic failover:
- Attempt to restart the offline Storage Pod
- Verify if the offline Storage host has returned to its normal state. If it has, the subsequent steps will be skipped
- Submit balance data job to remove offline Storage host
- Delete the offline storage Pod and its associated PVC, which will schedule the Pod on other nodes
- Submit balance data job to balance partitions to the newly created Pod


Here is the configuration file for NebulaCluster, which enables automatic failover in local PV scenarios.
```yaml
spec:
  # Enable failover
  enableAutoFailover: true
  # Duration for automatic failover after the storage host is offline
  # default 5m
  failoverPeriod: "2m"
```


