/*
Copyright 2023 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nebularestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	pb "github.com/vesoft-inc/nebula-agent/v3/pkg/proto"
	"github.com/vesoft-inc/nebula-agent/v3/pkg/storage"
	ng "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/annotation"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
	"github.com/vesoft-inc/nebula-operator/pkg/nebula"
	"github.com/vesoft-inc/nebula-operator/pkg/remote"
	"github.com/vesoft-inc/nebula-operator/pkg/util/async"
	"github.com/vesoft-inc/nebula-operator/pkg/util/condition"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
	rtutil "github.com/vesoft-inc/nebula-operator/pkg/util/restore"
)

type RestoreAgent struct {
	ctx        context.Context
	cfg        *rtutil.Config
	extStorage storage.ExternalStorage
	agentMgr   *nebula.AgentManager

	// bakMetas store backup meta list for restore
	bakMetas []*meta.BackupMeta

	rootURI    string
	backupName string

	metaDir      *ng.DirInfo
	storageHosts []*meta.ServiceInfo
}

type Manager interface {
	// Sync	implements the logic for syncing NebulaRestore.
	Sync(restore *v1alpha1.NebulaRestore) error
}

var _ Manager = (*restoreManager)(nil)

type restoreManager struct {
	clientSet     kube.ClientSet
	eventRecorder record.EventRecorder
}

func NewRestoreManager(clientSet kube.ClientSet, recorder record.EventRecorder) Manager {
	return &restoreManager{clientSet: clientSet, eventRecorder: recorder}
}

func (rm *restoreManager) Sync(restore *v1alpha1.NebulaRestore) error {
	return rm.syncRestoreProcess(restore)
}

// syncRestoreProcess restores nebula cluster which storage is one engine for one part
/*
backup_root/backup_name
  - meta
    - xxx.sst
    - ...
  - data
    - spaceId
      - partId
      - ...
    - ...
  - backup_name.meta
*/

func (rm *restoreManager) syncRestoreProcess(nr *v1alpha1.NebulaRestore) error {
	var ready bool
	ns := nr.GetNamespace()
	originalName := nr.Spec.Config.ClusterName
	if nr.Spec.Config.ClusterNamespace != nil {
		ns = *nr.Spec.Config.ClusterNamespace
	}
	original, err := rm.clientSet.NebulaCluster().GetNebulaCluster(ns, originalName)
	if err != nil {
		klog.Errorf("backup cluster [%s/%s] not found", ns, originalName)
		return err
	}

	restoredName, err := rm.getRestoredName(nr)
	if err != nil {
		return err
	}

	nc, _ := rm.clientSet.NebulaCluster().GetNebulaCluster(ns, restoredName)
	if nc != nil && annotation.IsInRestoreStage2(nc.Annotations) {
		if !nc.IsReady() {
			return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage2, waiting for cluster ready", ns, restoredName)
		}
		nc.Annotations = nil
		nc.Spec.Storaged.EnableForceUpdate = nil
		nc.Spec.EnableAutoFailover = original.Spec.EnableAutoFailover
		if err := rm.clientSet.NebulaCluster().UpdateNebulaCluster(nc); err != nil {
			return fmt.Errorf("remove cluster [%s/%s] annotations failed: %v", ns, restoredName, err)
		}

		return rm.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr,
			&v1alpha1.RestoreCondition{
				Type:   v1alpha1.RestoreComplete,
				Status: corev1.ConditionTrue,
			}, &kube.RestoreUpdateStatus{
				TimeCompleted: &metav1.Time{Time: time.Now()},
				ConditionType: v1alpha1.RestoreComplete,
				Partitions:    nil,
				Checkpoints:   nil,
			})
	}

	options, err := nebula.ClientOptions(original, nebula.SetIsMeta(true))
	if err != nil {
		return err
	}

	restored := rm.genNebulaCluster(restoredName, nr, original)
	if err := rm.clientSet.NebulaCluster().CreateNebulaCluster(restored); err != nil {
		return err
	}

	restoreAgent, err := initRestoreAgent(rm.clientSet, nr)
	if err != nil {
		return err
	}
	defer restoreAgent.agentMgr.Close()

	if err := rm.loadCluster(original, restored, restoreAgent, options); err != nil {
		return err
	}

	if err := os.MkdirAll(rtutil.LocalTmpDir, os.ModePerm); err != nil {
		return err
	}

	if err := restoreAgent.loadBakMetas(restoreAgent.backupName); err != nil {
		return err
	}
	klog.Infof("[%s/%s] load backup metad file successfully", ns, restoredName)

	if err := restoreAgent.checkTopology(); err != nil {
		return err
	}

	if !condition.IsRestoreMetadComplete(nr) {
		if !rm.endpointsConnected(restoreAgent, restored.GetMetadEndpoints(v1alpha1.MetadPortNameThrift)) {
			return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage1, waiting for metad init agent are connected", ns, restoredName)
		}

		if err := restoreAgent.downloadMetaData(restored.GetMetadEndpoints(v1alpha1.MetadPortNameThrift)); err != nil {
			klog.Errorf("download metad files failed: %v", err)
			return err
		}

		klog.Infof("restoring [%s/%s] in stage1, download metad files successfully", ns, restoredName)

		ready, err = rm.metadReady(ns, restoredName)
		if err != nil {
			return err
		}
		if !ready {
			return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage1, waiting for metad running", ns, restoredName)
		}

		hostPairs := restoreAgent.genHostPairs(restoreAgent.bakMetas[0], restored.GetStoragedEndpoints(v1alpha1.StoragedPortNameThrift))
		metaEndpoints := restored.GetMetadEndpoints(v1alpha1.MetadPortNameThrift)
		restoreResp, err := restoreAgent.restoreMeta(restoreAgent.bakMetas[0], hostPairs, metaEndpoints, options)
		if err != nil {
			klog.Errorf("restore metad data [%s/%s] failed, error: %v", ns, restoredName, err)
			return err
		}

		klog.Infof("restoring [%s/%s] in stage1, restore metad data successfully", ns, restoredName)

		hostPartMap := rtutil.FlattenRestoreMeta(restoreResp)

		rtutil.FlattenRestoreMeta(restoreResp)

		if err := rm.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr,
			&v1alpha1.RestoreCondition{
				Type:   v1alpha1.RestoreMetadComplete,
				Status: corev1.ConditionTrue,
			}, &kube.RestoreUpdateStatus{
				Partitions:    hostPartMap,
				ConditionType: v1alpha1.RestoreMetadComplete,
			}); err != nil {
			return err
		}
	}

	if err := rm.updateClusterAnnotations(ns,
		restoredName,
		map[string]string{annotation.AnnRestoreMetadStepKey: annotation.AnnRestoreMetadStepVal}); err != nil {
		return err
	}

	if !condition.IsRestoreStoragedComplete(nr) {
		if !rm.endpointsConnected(restoreAgent, restored.GetStoragedEndpoints(v1alpha1.StoragedPortNameThrift)) {
			return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage1, waiting for storaged init agent are connected", ns, restoredName)
		}

		checkpoints, err := restoreAgent.downloadStorageData(nr.Status.Partitions, restoreAgent.storageHosts)
		if err != nil {
			klog.Errorf("download storaged files failed: %v", err)
			return err
		}

		klog.Infof("restoring [%s/%s] in stage1, download storaged files successfully", ns, restoredName)

		if err := restoreAgent.playBackStorageData(restored.GetMetadEndpoints(v1alpha1.MetadPortNameThrift), restoreAgent.storageHosts); err != nil {
			return err
		}

		klog.Infof("restoring [%s/%s] in stage1, playback storaged data successfully", ns, restoredName)

		if err := rm.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr,
			&v1alpha1.RestoreCondition{
				Type:   v1alpha1.RestoreStoragedCompleted,
				Status: corev1.ConditionTrue,
			}, &kube.RestoreUpdateStatus{
				Checkpoints:   checkpoints,
				ConditionType: v1alpha1.RestoreStoragedCompleted,
			}); err != nil {
			return err
		}
	}

	if err := rm.updateClusterAnnotations(ns,
		restoredName,
		map[string]string{annotation.AnnRestoreStoragedStepKey: annotation.AnnRestoreStoragedStepVal}); err != nil {
		return err
	}

	ready, err = rm.clusterReady(ns, restoredName)
	if err != nil {
		return err
	}
	if !ready {
		return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage1, waiting for cluster ready", ns, restoredName)
	}

	if !rm.endpointsConnected(restoreAgent, restored.GetStoragedEndpoints(v1alpha1.StoragedPortNameThrift)) {
		return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage1, waiting for storaged sidecar agent are connected", ns, restoredName)
	}

	if err := restoreAgent.removeDownloadCheckpoints(nr.Status.Checkpoints); err != nil {
		klog.Errorf("remove downloaded checkpoints failed: %v", err)
		return err
	}
	klog.Infof("restoring [%s/%s] in stage1, remove download checkpoints successfully", ns, restoredName)

	if err := rm.removeInitAgentContainer(ns, restoredName); err != nil {
		klog.Errorf("remove init agent containers failed: %v", err)
		return err
	}
	klog.Infof("restoring [%s/%s] in stage1, remove init agent container successfully", ns, restoredName)

	return utilerrors.ReconcileErrorf("restoring [%s/%s] in stage2, waiting for cluster ready", ns, restoredName)
}

func (rm *restoreManager) loadCluster(original, restored *v1alpha1.NebulaCluster, restoreAgent *RestoreAgent, options []nebula.Option) error {
	mc, err := nebula.NewMetaClient([]string{original.GetMetadThriftConnAddress()}, options...)
	if err != nil {
		return err
	}

	resp, err := mc.ListCluster()
	if err != nil {
		return err
	}

	hosts := &rtutil.NebulaHosts{}
	if err := hosts.LoadFrom(resp); err != nil {
		return err
	}

	restoreAgent.metaDir = hosts.GetMetas()[0].Dir
	rm.replaceStorageHosts(hosts.GetStorages(), restored, restoreAgent)

	klog.Infof("restore metad dir info, root: %s data: %s", string(restoreAgent.metaDir.Root), string(restoreAgent.metaDir.Data[0]))

	return nil
}

func (rm *restoreManager) replaceStorageHosts(original []*meta.ServiceInfo, restored *v1alpha1.NebulaCluster, restoreAgent *RestoreAgent) {
	for i := range original {
		original[i].Addr.Host = restored.StoragedComponent().GetPodFQDN(int32(i))
	}
	restoreAgent.storageHosts = original
}

func (rm *restoreManager) genNebulaCluster(restoredName string, nr *v1alpha1.NebulaRestore, original *v1alpha1.NebulaCluster) *v1alpha1.NebulaCluster {
	annotations := map[string]string{
		annotation.AnnRestoreNameKey:  nr.Name,
		annotation.AnnRestoreStageKey: annotation.AnnRestoreStage1Val,
	}
	nc := &v1alpha1.NebulaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        restoredName,
			Namespace:   original.Namespace,
			Annotations: annotations,
		},
		Spec: original.Spec,
	}

	nc.Spec.Metad.InitContainers = append(nc.Spec.Metad.InitContainers, v1alpha1.GenerateInitAgentContainer(nc.MetadComponent()))
	nc.Spec.Storaged.InitContainers = append(nc.Spec.Storaged.InitContainers, v1alpha1.GenerateInitAgentContainer(nc.StoragedComponent()))

	nc.Spec.Storaged.EnableForceUpdate = pointer.Bool(true)
	nc.Spec.EnableAutoFailover = nil
	if nr.Spec.NodeSelector != nil {
		nc.Spec.NodeSelector = nr.Spec.NodeSelector
	}
	if nr.Spec.Affinity != nil {
		nc.Spec.Affinity = nr.Spec.Affinity
	}
	if nr.Spec.Tolerations != nil {
		nc.Spec.Tolerations = nr.Spec.Tolerations
	}

	return nc
}

func initRestoreAgent(clientSet kube.ClientSet, restore *v1alpha1.NebulaRestore) (*RestoreAgent, error) {
	backend := &pb.Backend{}
	storageType := remote.GetStorageType(restore.Spec.Config.StorageProvider)
	switch storageType {
	case v1alpha1.ObjectStorageS3:
		if err := backend.SetUri(fmt.Sprintf("s3://%s", restore.Spec.Config.S3.Bucket)); err != nil {
			return nil, err
		}
		backend.GetS3().Region = restore.Spec.Config.S3.Region
		backend.GetS3().Endpoint = restore.Spec.Config.S3.Endpoint
		accessKey, secretKey, err := remote.GetS3Key(clientSet, restore.Namespace, restore.Spec.Config.S3.SecretName)
		if err != nil {
			return nil, fmt.Errorf("get S3 key failed: %v", err)
		}
		backend.GetS3().AccessKey = accessKey
		backend.GetS3().SecretKey = secretKey
	case v1alpha1.ObjectStorageGS:
		if err := backend.SetUri(fmt.Sprintf("gs://%s", restore.Spec.Config.GS.Bucket)); err != nil {
			return nil, err
		}
		credentials, err := remote.GetGsCredentials(clientSet, restore.Namespace, restore.Spec.Config.GS.SecretName)
		if err != nil {
			return nil, fmt.Errorf("get GS credentials failed: %v", err)
		}
		backend.GetGs().Credentials = credentials
	default:
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}

	cfg := &rtutil.Config{
		BackupName:  restore.Spec.Config.BackupName,
		Concurrency: restore.Spec.Config.Concurrency,
		Backend:     backend,
	}

	ra, err := newRestoreAgent(cfg)
	if err != nil {
		return nil, err
	}

	return ra, nil
}

func newRestoreAgent(cfg *rtutil.Config) (*RestoreAgent, error) {
	extStorage, err := storage.New(cfg.Backend)
	if err != nil {
		return nil, err
	}

	return &RestoreAgent{
		ctx:        context.TODO(),
		cfg:        cfg,
		extStorage: extStorage,
		agentMgr:   nebula.NewAgentManager(),
		rootURI:    cfg.Backend.Uri(),
		backupName: cfg.BackupName,
	}, nil
}

func (r *RestoreAgent) checkTopology() error {
	if len(r.bakMetas[0].GetStorageHosts()) != len(r.storageHosts) {
		return fmt.Errorf("the cluster topology of storaged count must be consistent")
	}

	return nil
}

func (r *RestoreAgent) loadBakMetas(backupName string) error {
	// check backup dir existence
	rootURI, err := rtutil.UriJoin(r.cfg.Backend.Uri(), backupName)
	if err != nil {
		return err
	}
	exist := r.extStorage.ExistDir(r.ctx, rootURI)
	if !exist {
		return fmt.Errorf("backup dir %s does not exist", rootURI)
	}

	// download and parse backup meta file
	backupMetaName := fmt.Sprintf("%s.meta", backupName)
	metaUri, _ := rtutil.UriJoin(rootURI, backupMetaName)
	tmpPath := filepath.Join(rtutil.LocalTmpDir, backupMetaName)
	if err := r.extStorage.Download(r.ctx, tmpPath, metaUri, false); err != nil {
		return fmt.Errorf("download %s to %s failed: %v", metaUri, tmpPath, err)
	}

	bakMeta, err := rtutil.ParseMetaFromFile(tmpPath)
	if err != nil {
		return fmt.Errorf("parse backup metad file %s failed: %v", tmpPath, err)
	}

	r.bakMetas = append(r.bakMetas, bakMeta)

	if len(bakMeta.BaseBackupName) > 0 {
		return r.loadBakMetas(string(bakMeta.BaseBackupName))
	}

	return nil
}

func (r *RestoreAgent) downloadMetaData(metaEndpoints []string) error {
	// {backupRoot}/{backupName}/meta/*.sst
	externalUri, _ := rtutil.UriJoin(r.rootURI, r.backupName, "meta")
	backend, err := r.extStorage.GetDir(r.ctx, externalUri)
	if err != nil {
		return err
	}

	// download meta backup files to every meta service
	for _, ep := range metaEndpoints {
		addr, err := rtutil.ParseAddr(ep)
		if err != nil {
			return err
		}
		agent, err := r.agentMgr.GetAgent(addr)
		if err != nil {
			return err
		}

		// meta kv data path: {nebulaData}/meta
		req := &pb.DownloadFileRequest{
			SourceBackend: backend,
			TargetPath:    string(r.metaDir.Data[0]),
			Recursively:   true,
		}
		if _, err := agent.DownloadFile(req); err != nil {
			return err
		}
		if _, err := agent.StopAgent(&pb.StopAgentRequest{}); err != nil {
			return err
		}
	}

	return nil
}

func (r *RestoreAgent) downloadStorageData(parts map[string][]*ng.HostAddr, storageHosts []*meta.ServiceInfo) (map[string]map[string]string, error) {
	// checkpoints save the download checkpoint paths, make cleanup restore data easier
	checkpoints := make(map[string]map[string]string)

	// dataPathSelector is a selector map which select current hosts' dataPath on average
	dataPathSelector := rtutil.NewPathSelectorMap(storageHosts)
	dataPathMap := make(map[string]string)
	group := async.NewGroup(context.TODO(), r.cfg.Concurrency)
	for _, backupMeta := range r.bakMetas {
		storageUri, _ := rtutil.UriJoin(r.rootURI, string(backupMeta.GetBackupName()), "data")
		partMap := rtutil.FlattenBackupMeta(backupMeta)
		for _, info := range partMap {
			// restore one part to multiple part
			key := rtutil.GenPartKey(info.SpaceID, info.PartID)
			for _, host := range parts[key] {
				agent, err := r.agentMgr.GetAgent(host)
				if err != nil {
					return nil, err
				}

				externalUri, _ := rtutil.UriJoin(storageUri, info.SpaceID, info.PartID)
				// avoid agent.DownloadFile prefix bugs
				externalUri += "/"

				source, err := r.extStorage.GetDir(r.ctx, externalUri)
				if err != nil {
					return nil, err
				}

				// ensure every part's all checkpoint place in same dataPath
				dataPathKey := rtutil.GenDataPathKey(host, key)
				dataPath, hasPlace := dataPathMap[dataPathKey]
				if !hasPlace {
					dataPath = dataPathSelector[rtutil.StringifyAddr(host)].EvenlyGet()
					dataPathMap[dataPathKey] = dataPath
				}

				targetCk := filepath.Join(dataPath, "nebula", info.SpaceID, info.PartID, "checkpoints")
				target := filepath.Join(targetCk, string(backupMeta.BackupName))

				// source: {backupRoot}/{backupName}/data/{spaceID}/{partID}/
				// target: {nebulaDataPath}/nebula/{spaceID}/{partID}/checkpoints/{backupName}/
				req := &pb.DownloadFileRequest{
					SourceBackend: source,
					TargetPath:    target,
					Recursively:   true,
				}

				if _, ok := checkpoints[rtutil.StringifyAddr(host)]; !ok {
					checkpoints[rtutil.StringifyAddr(host)] = make(map[string]string)
				}
				checkpoints[rtutil.StringifyAddr(host)][targetCk] = ""

				worker := func() error {
					if _, err := agent.DownloadFile(req); err != nil {
						return err
					}
					return nil
				}

				group.Add(func(stopCh chan interface{}) {
					stopCh <- worker()
				})
			}
		}
	}

	return checkpoints, group.Wait()
}

func (r *RestoreAgent) restoreMeta(backup *meta.BackupMeta, storageMap map[string]string, metaEndpoints []string, options []nebula.Option) (*meta.RestoreMetaResp, error) {
	addrMap := make([]*meta.HostPair, 0, len(storageMap))
	for from, to := range storageMap {
		fromAddr, err := rtutil.ParseAddr(from)
		if err != nil {
			return nil, err
		}
		toAddr, err := rtutil.ParseAddr(to)
		if err != nil {
			return nil, err
		}
		addrMap = append(addrMap, &meta.HostPair{FromHost: fromAddr, ToHost: toAddr})
	}

	var restoreRes *meta.RestoreMetaResp
	for _, metaEndpoint := range metaEndpoints {
		files := make([]string, 0, len(backup.GetMetaFiles()))
		for _, file := range backup.GetMetaFiles() {
			filePath := fmt.Sprintf("%s/%s", string(r.metaDir.Data[0]), string(file))
			files = append(files, filePath)
		}

		mc, err := nebula.NewMetaClient([]string{metaEndpoint}, options...)
		if err != nil {
			if utilerrors.IsDNSError(err) {
				return nil, utilerrors.ReconcileErrorf("waiting for %s dns lookup is ok", metaEndpoint)
			}
			return nil, err
		}

		resp, err := mc.RestoreMeta(addrMap, files)
		klog.Infof("restore metad sst files %v", files)
		if err != nil {
			return nil, fmt.Errorf("restore metad service %s failed: %v", metaEndpoint, err)
		}
		if restoreRes == nil {
			restoreRes = resp
		}

		if err := mc.Disconnect(); err != nil {
			klog.Error(err)
		}

		klog.Infof("restore backup data on metad %s successfully", metaEndpoint)
	}

	return restoreRes, nil
}

// genHostPairs generate old:new storage host pairs
func (r *RestoreAgent) genHostPairs(backup *meta.BackupMeta, restoreHosts []string) map[string]string {
	hostPairs := make(map[string]string)
	backupHosts := make([]string, 0)

	for _, st := range backup.GetStorageHosts() {
		addr := rtutil.StringifyAddr(st)
		backupHosts = append(backupHosts, addr)
	}

	for i := range restoreHosts {
		hostPairs[backupHosts[i]] = restoreHosts[i]
	}

	klog.Infof("restore meta hostPairs %v", hostPairs)

	return hostPairs
}

func (r *RestoreAgent) playBackStorageData(metaEndpoints []string, storageHosts []*meta.ServiceInfo) error {
	group := async.NewGroup(context.TODO(), r.cfg.Concurrency)
	for i := range storageHosts {
		s := storageHosts[i]
		agent, err := r.agentMgr.GetAgent(s.GetAddr())
		if err != nil {
			return err
		}

		dataPaths := make([]string, 0)
		for _, d := range s.GetDir().GetData() {
			dataPaths = append(dataPaths, string(d))
		}

		req := &pb.DataPlayBackRequest{
			Dir:      string(s.GetDir().GetRoot()),
			DataPath: strings.Join(dataPaths, ","),
			MetaAddr: strings.Join(metaEndpoints, ","),
		}

		worker := func() error {
			_, err = agent.DataPlayBack(req)
			if err != nil {
				return fmt.Errorf("storaged data playback failed: %v", err)
			}
			klog.Infof("backup data playback on storaged %s successfully", s.GetAddr().GetHost())
			return nil
		}

		group.Add(func(stopCh chan interface{}) {
			stopCh <- worker()
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	for _, s := range storageHosts {
		agent, err := r.agentMgr.GetAgent(s.GetAddr())
		if err != nil {
			return err
		}
		if _, err := agent.StopAgent(&pb.StopAgentRequest{}); err != nil {
			return fmt.Errorf("stop agent failed: %v", err)
		}
	}

	return nil
}

func (r *RestoreAgent) removeDownloadCheckpoints(checkpoints map[string]map[string]string) error {
	for addr, paths := range checkpoints {
		host, err := rtutil.ParseAddr(addr)
		if err != nil {
			return err
		}
		agent, err := r.agentMgr.GetAgent(host)
		if err != nil {
			return fmt.Errorf("get agent for storaged %s failed: %v", addr, err)
		}
		for path := range paths {
			if _, err = agent.RemoveDir(&pb.RemoveDirRequest{Path: path}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (rm *restoreManager) removeInitAgentContainer(namespace, ncName string) error {
	updated, err := rm.clientSet.NebulaCluster().GetNebulaCluster(namespace, ncName)
	if err != nil {
		return err
	}

	updated.SetAnnotations(map[string]string{annotation.AnnRestoreStageKey: annotation.AnnRestoreStage2Val})

	m := make([]corev1.Container, 0)
	for _, c := range updated.Spec.Metad.InitContainers {
		if c.Name == v1alpha1.AgentInitContainerName {
			continue
		}
		m = append(m, c)
	}
	updated.Spec.Metad.InitContainers = m

	s := make([]corev1.Container, 0)
	for _, c := range updated.Spec.Storaged.InitContainers {
		if c.Name == v1alpha1.AgentInitContainerName {
			continue
		}
		s = append(s, c)
	}
	updated.Spec.Storaged.InitContainers = s

	return rm.clientSet.NebulaCluster().UpdateNebulaCluster(updated)
}

func (rm *restoreManager) updateClusterAnnotations(namespace, ncName string, annotations map[string]string) error {
	updated, err := rm.clientSet.NebulaCluster().GetNebulaCluster(namespace, ncName)
	if err != nil {
		return err
	}

	var needUpdate bool
	if updated.GetAnnotations() == nil {
		updated.SetAnnotations(annotations)
		needUpdate = true
	} else {
		for annKey, annVal := range annotations {
			v, exists := updated.Annotations[annKey]
			if exists && annVal == v {
				continue
			}
			updated.Annotations[annKey] = annVal
			needUpdate = true
		}
	}

	if needUpdate {
		klog.Infof("NebulaCluster [%s/%s] will update annotations %v", namespace, ncName, annotations)
		return rm.clientSet.NebulaCluster().UpdateNebulaCluster(updated)
	}

	return nil
}

func (rm *restoreManager) endpointsConnected(restoreAgent *RestoreAgent, endpoints []string) bool {
	for _, ep := range endpoints {
		host, err := rtutil.ParseAddr(ep)
		if err != nil {
			klog.Error(err)
			return false
		}
		agent, err := restoreAgent.agentMgr.GetAgent(host)
		if err != nil {
			return false
		}
		resp, err := agent.HealthCheck(&pb.HealthCheckRequest{})
		if err != nil {
			return false
		}
		if resp != nil && resp.Status != "healthy" {
			return false
		}
	}

	return true
}

func (rm *restoreManager) metadReady(namespace, ncName string) (bool, error) {
	nc, err := rm.clientSet.NebulaCluster().GetNebulaCluster(namespace, ncName)
	if err != nil {
		return false, err
	}
	return nc.MetadComponent().IsReady(), nil
}

func (rm *restoreManager) clusterReady(namespace, ncName string) (bool, error) {
	nc, err := rm.clientSet.NebulaCluster().GetNebulaCluster(namespace, ncName)
	if err != nil {
		return false, err
	}
	return nc.IsReady(), nil
}

func (rm *restoreManager) getRestoredName(nr *v1alpha1.NebulaRestore) (string, error) {
	if nr.Status.ClusterName == "" {
		genName := "ng" + rand.String(4)
		newStatus := &kube.RestoreUpdateStatus{
			TimeStarted: &metav1.Time{Time: time.Now()},
			ClusterName: pointer.String(genName),
		}
		if err := rm.clientSet.NebulaRestore().UpdateNebulaRestoreStatus(nr, nil, newStatus); err != nil {
			return "", err
		}

		klog.Infof("generate [%s/%s] restored nebula cluster name successfully", nr.Namespace, nr.Name)
		return genName, nil
	}

	return nr.Status.ClusterName, nil
}

func getPodTerminateReason(pod corev1.Pod) string {
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("container %s terminated: %s", cs.Name, cs.State.Terminated.String())
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return fmt.Sprintf("container %s terminated: %s", cs.Name, cs.State.Terminated.String())
		}
	}
	return ""
}
