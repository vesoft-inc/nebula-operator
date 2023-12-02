## Scale storage nodes and balance

Scaling out Storaged is divided into two stages. 

* In the first stage, you need to wait for the status of all newly created Pods to be Ready. 

* In the second stage, the BALANCE DATA and BALANCE LEADER command is executed. 

We provide a field `enableAutoBalance` in CRD to control whether to automatically balance data and leader.
Through both stages, the scaling process of the controller replicas is decoupled from the balancing data process and user executing it at low traffic.
Such an implementation can effectively reduce the impact of data migration on online services, which is in line with the NebulaGraph principle: Balancing data is not fully automated and when to balance data is decided by users.

## Storage leaders transfer

When NebulaGraph starts to provide services, there will be multiple partition leaders on each Storaged node, 
in the rolling update scenario, in order to minimize the impact on client reads and writes, 
so it is necessary to transfer the leader to other nodes until the number of leaders on a storage node is 0, this process is relatively long.

To make rolling updates more convenient for DBA, we provide a field `enableForceUpdate` for Storaged service, 
which can directly roll update storage when it is determined that there is no external access traffic, 
without waiting for the partition leader to be transferred before it can operate.