# PV Expansion

Volume expansion was introduced as an alpha feature in Kubernetes 1.8, and it went beta in 1.11 and with Kubernetes 1.24 
we are excited to announce general availability(GA) of volume expansion.

This feature allows Kubernetes users to simply edit their PersistentVolumeClaim objects and specify new size in PVC Spec 
and Kubernetes will automatically expand the volume using storage backend and also expand the underlying file system in-use
by the Pod without requiring any downtime at all if possible.

Not every volume type however is expandable by default. Some volume types such as - intree hostpath volumes are not expandable at all. 
For CSI volumes - the CSI driver must have capability EXPAND_VOLUME in controller or node service (or both if appropriate). 
Please refer to volume expansion documentation for intree volume types which support volume expansion - [Expanding Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims).

In general to provide some degree of control over volumes that can be expanded, only dynamically provisioned PVCs whose storage class 
has allowVolumeExpansion parameter set to true are expandable.

A Kubernetes cluster administrator must edit the appropriate StorageClass object and set the allowVolumeExpansion field to true. For example:
```shell
$ kubectl patch storageclass ebs-sc -p '{"allowVolumeExpansion": true}'
```

After allowVolumeExpansion is enabled, perform the following operations to expand PV capacity:
```shell
$ kubectl patch nc nebula --type='merge' --patch '{"spec": {"storaged": {"dataVolumeClaims":[{"resources": {"requests": {"storage": "100Gi"}}, "storageClassName": "ebs-sc"}]}}}'
```

After the expansion is successful, run `kubectl get pvc -n <namespace> <pvc-name>` to display the original size, but viewing the PV size will show that it has expanded to the expected size.
```shell
kubectl get pv | grep <pvc-name>
```
