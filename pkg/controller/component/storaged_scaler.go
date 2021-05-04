package component

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nebulago "github.com/vesoft-inc/nebula-go/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	controllerutil "github.com/vesoft-inc/nebula-operator/pkg/util/controller"
)

type storageScaler struct {
	client.Client
	clientSet kube.ClientSet
	extender  controllerutil.UnstructuredExtender
}

func NewStorageScaler(cli client.Client, clientSet kube.ClientSet) ScaleManager {
	return &storageScaler{
		cli,
		clientSet,
		&controllerutil.Unstructured{},
	}
}

func (ss *storageScaler) Scale(nc *v1alpha1.NebulaCluster, oldUnstruct, newUnstruct *unstructured.Unstructured) error {
	oldReplicas := ss.extender.GetReplicas(oldUnstruct)
	newReplicas := ss.extender.GetReplicas(newUnstruct)

	if *newReplicas < *oldReplicas || nc.Status.Storaged.Phase == v1alpha1.ScaleInPhase {
		return ss.ScaleIn(nc, *oldReplicas, *newReplicas)
	}

	if *newReplicas > *oldReplicas || nc.Status.Storaged.Phase == v1alpha1.ScaleOutPhase {
		return ss.ScaleOut(nc)
	}

	return nil
}

func (ss *storageScaler) ScaleOut(nc *v1alpha1.NebulaCluster) error {
	nc.Status.Storaged.Phase = v1alpha1.ScaleOutPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	if !nc.StoragedComponent().IsReady() {
		klog.InfoS("storage cluster status not ready", "storage",
			klog.KRef(nc.Namespace, nc.StoragedComponent().GetName()))
		return nil
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints)
	if err != nil {
		klog.ErrorS(err, "create meta client failed", "endpoints", endpoints)
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			klog.ErrorS(err, "disconnect meta client failed", "endpoints", endpoints)
		}
	}()

	if err := metaClient.BalanceData(); err != nil {
		return err
	}

	if err := metaClient.BalanceLeader(); err != nil {
		klog.ErrorS(err, "unable to balance leader")
		return err
	}

	nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	return nil
}

func (ss *storageScaler) ScaleIn(nc *v1alpha1.NebulaCluster, oldReplicas, newReplicas int32) error {
	nc.Status.Storaged.Phase = v1alpha1.ScaleInPhase
	if err := ss.clientSet.NebulaCluster().UpdateNebulaClusterStatus(nc.DeepCopy()); err != nil {
		return err
	}

	endpoints := []string{nc.GetMetadThriftConnAddress()}
	metaClient, err := nebula.NewMetaClient(endpoints)
	if err != nil {
		return err
	}
	defer func() {
		err := metaClient.Disconnect()
		if err != nil {
			klog.Warningf("meta client disconnect %s", err)
		}
	}()

	if oldReplicas-newReplicas > 0 {
		hosts := make([]*nebulago.HostAddr, 0, oldReplicas-newReplicas)
		port := nebulago.Port(nc.StoragedComponent().GetPort(v1alpha1.StoragedPortNameThrift))
		for i := oldReplicas - 1; i >= newReplicas; i-- {
			hosts = append(hosts, &nebulago.HostAddr{
				Host: nc.StoragedComponent().GetPodFQDN(i),
				Port: port,
			})
		}
		if len(hosts) > 0 {
			if err := metaClient.RemoveHost(hosts); err != nil {
				return err
			}
		}
	}

	if err := PvcMark(ss.clientSet.PVC(), nc.StoragedComponent(), oldReplicas, newReplicas); err != nil {
		return err
	}

	var deleted bool
	pvcName := ordinalPVCName(nc.StoragedComponent().Type(), nc.StoragedComponent().GetName(), newReplicas)
	_, err = ss.clientSet.PVC().GetPVC(nc.GetNamespace(), pvcName)
	if apierrors.IsNotFound(err) {
		deleted = true
	}

	if deleted && nc.StoragedComponent().IsReady() {
		klog.InfoS("all used pvcs were reclaimed", "storage",
			klog.KRef(nc.Namespace, nc.StoragedComponent().GetName()))
		nc.Status.Storaged.Phase = v1alpha1.RunningPhase
	}

	return nil
}
