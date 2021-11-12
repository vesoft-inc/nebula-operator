# PV reclaim

Nebula Operator uses PV (Persistent Volume) and PVC (Persistent Volume Claim) to store persistent data. If you accidentally delete a nebula cluster, the PV/PVC objects and data are still retained to ensure data safety.

We provide a parameter `enablePVReclaim` in crd to control whether reclaim the pv or not.

If you need release the storage spaces and don't want to retain the data, you can update your nebula instance and set the parameter `enablePVReclaim` to __true__.
