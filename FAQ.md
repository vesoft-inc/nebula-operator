- Does Nebula Operator support v1.x ?

V1.x does not support DNS and Nebula Operator requires this feature, it's not compatible.

- When to support upgrading feature ?

The Nebula Operator needs to be consistent with the NebulaGraph, after the database supports rolling upgrades, the Operator will synchronize support for this feature.

- Whether guarantees cluster stability if using local storage ?

There is no guarantee that using local storage, it means that the POD is bound to a particular node. Operators do not currently have the ability to failover when the bound node goes down. This is not an issue with network storage. 

- How to ensure the stable availability of scaling cluster ?

It is recommended to backup data in advance, so as you can go back if failed. 

