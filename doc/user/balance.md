## Scale storage nodes and Balance

Scaling out Storage is divided into two stages. 

* In the first stage, you need to wait for the status of all newly created Pods to be Ready. 

* In the second stage, the BALANCE DATA and BALANCE LEADER command is executed. 

We provide a parameter `enableAutoBalance` in CRD to control whether to automatically balance data and leader.

Through both stages, the scaling process of the controller replicas is decoupled from the balancing data process and user executing it at low traffic. 

Such an implementation can effectively reduce the impact of data migration on online services, which is in line with the NebulaGraph principle: Balancing data is not fully automated and when to balance data is decided by users.