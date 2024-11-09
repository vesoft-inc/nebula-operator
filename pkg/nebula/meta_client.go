/*
Copyright 2021 Vesoft Inc.

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

package nebula

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

var (
	ErrNoAvailableMetadEndpoints = errors.New("metadclient: no available hosts")
	ErrJobStatusFailed           = errors.New("job status failed")
)

var _ MetaInterface = (*metaClient)(nil)

type (
	Fn func(req interface{}) (interface{}, error)

	MetaInterface interface {
		GetSpace(spaceName []byte) (*meta.SpaceItem, error)
		DropSpace(spaceName []byte) error
		ListSpaces() ([]*meta.IdName, error)
		ListCluster() (*meta.ListClusterInfoResp, error)
		AddHosts(hosts []*nebula.HostAddr) error
		AddHostsIntoZone(hosts []*nebula.HostAddr, zone string) error
		DropHosts(hosts []*nebula.HostAddr) error
		ListHosts(hostType meta.ListHostType) ([]*meta.HostItem, error)
		ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error)
		GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error)
		GetSpaceLeaderHosts(space []byte) ([]string, error)
		GetLeaderCount(leaderHost string) (int, error)
		BalanceStatus(jobID int32, spaceID nebula.GraphSpaceID) error
		IsLeaderBalanced(spaceName []byte) (bool, error)
		BalanceLeader(spaceID nebula.GraphSpaceID) error
		BalanceData(spaceID nebula.GraphSpaceID) (int32, error)
		BalanceDataInZone(spaceID nebula.GraphSpaceID) (int32, error)
		RemoveHost(spaceID nebula.GraphSpaceID, hosts []*nebula.HostAddr) (int32, error)
		RemoveHostInZone(spaceID nebula.GraphSpaceID, hosts []*nebula.HostAddr) (int32, error)
		RecoverDataBalanceJob(jobID int32, spaceID nebula.GraphSpaceID) error
		RecoverInZoneBalanceJob(jobID int32, spaceID nebula.GraphSpaceID) error
		RestoreMeta(hosts []*meta.HostPair, files []string) (*meta.RestoreMetaResp, error)
		Disconnect() error
	}

	metaClient struct {
		client  *meta.MetaServiceClient
		options []Option
	}
)

func NewMetaClient(hosts []string, options ...Option) (MetaInterface, error) {
	if len(hosts) == 0 {
		return nil, ErrNoAvailableMetadEndpoints
	}
	var err error
	var mc MetaInterface
	for i := 0; i < len(hosts); i++ {
		mc, err = newMetaConnection(hosts[i], options...)
		if err != nil {
			klog.Error(err)
			continue
		}
	}
	if err != nil {
		return nil, err
	}
	return mc, nil
}

func newMetaConnection(endpoint string, options ...Option) (*metaClient, error) {
	transport, pf, err := buildClientTransport(endpoint, options...)
	if err != nil {
		return nil, err
	}
	metaServiceClient := meta.NewMetaServiceClientFactory(transport, pf)
	mc := &metaClient{
		client:  metaServiceClient,
		options: options,
	}
	if err := mc.connect(); err != nil {
		return nil, err
	}
	return mc, nil
}

func (m *metaClient) reconnect(endpoint string) error {
	if err := m.client.Close(); err != nil {
		return err
	}

	transport, pf, err := buildClientTransport(endpoint, m.options...)
	if err != nil {
		return err
	}

	metaServiceClient := meta.NewMetaServiceClientFactory(transport, pf)
	if err := metaServiceClient.Open(); err != nil {
		return err
	}

	if !metaServiceClient.IsOpen() {
		return errors.Errorf("transport is not open")
	}
	m.client = metaServiceClient

	return nil
}

func (m *metaClient) connect() error {
	if err := m.client.Open(); err != nil {
		return err
	}
	if !m.client.IsOpen() {
		return errors.Errorf("transport is not open")
	}
	return nil
}

func (m *metaClient) Disconnect() error {
	return m.client.Close()
}

func (m *metaClient) GetSpace(spaceName []byte) (*meta.SpaceItem, error) {
	req := &meta.GetSpaceReq{SpaceName: spaceName}
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.GetSpace(req.(*meta.GetSpaceReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.GetSpaceResp).Item, nil
}

func (m *metaClient) DropSpace(spaceName []byte) error {
	req := &meta.DropSpaceReq{SpaceName: spaceName, IfExists: true}
	_, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.DropSpace(req.(*meta.DropSpaceReq))
		return resp, err
	})
	return err
}

func (m *metaClient) ListSpaces() ([]*meta.IdName, error) {
	resp, err := m.retryOnError(meta.NewListSpacesReq(), func(req interface{}) (interface{}, error) {
		resp, err := m.client.ListSpaces(req.(*meta.ListSpacesReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.ListSpacesResp).Spaces, nil
}

func (m *metaClient) ListCluster() (*meta.ListClusterInfoResp, error) {
	r, err := m.retryOnError(meta.NewListClusterInfoReq(), func(req interface{}) (interface{}, error) {
		resp, err := m.client.ListCluster(req.(*meta.ListClusterInfoReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	resp := r.(*meta.ListClusterInfoResp)
	for _, services := range resp.GetHostServices() {
		for _, s := range services {
			if s.Role == meta.HostRole_META {
				dir, err := m.getMetaDirInfo()
				if err != nil {
					return nil, err
				}
				s.Dir = dir
			}
		}
	}
	return resp, nil
}

func (m *metaClient) getMetaDirInfo() (*nebula.DirInfo, error) {
	resp, err := m.client.GetMetaDirInfo(meta.NewGetMetaDirInfoReq())
	if err != nil {
		return nil, err
	}

	return resp.GetDir(), nil
}

func (m *metaClient) AddHosts(hosts []*nebula.HostAddr) error {
	req := &meta.AddHostsReq{
		Hosts: hosts,
	}
	_, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.AddHosts(req.(*meta.AddHostsReq))
		return resp, err
	})
	return err
}

func (m *metaClient) AddHostsIntoZone(hosts []*nebula.HostAddr, zone string) error {
	req := &meta.AddHostsIntoZoneReq{
		Hosts:    hosts,
		ZoneName: []byte(zone),
	}
	_, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.AddHostsIntoZone(req.(*meta.AddHostsIntoZoneReq))
		return resp, err
	})
	return err
}

func (m *metaClient) DropHosts(hosts []*nebula.HostAddr) error {
	req := &meta.DropHostsReq{
		Hosts: hosts,
	}
	_, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.DropHosts(req.(*meta.DropHostsReq))
		return resp, err
	})
	return err
}

func (m *metaClient) ListHosts(hostType meta.ListHostType) ([]*meta.HostItem, error) {
	req := &meta.ListHostsReq{Type: hostType}
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.ListHosts(req.(*meta.ListHostsReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.ListHostsResp).Hosts, nil
}

func (m *metaClient) ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error) {
	req := &meta.ListPartsReq{
		SpaceID: spaceID,
		PartIds: partIDs,
	}
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.ListParts(req.(*meta.ListPartsReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.ListPartsResp).Parts, nil
}

func (m *metaClient) GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error) {
	spaceParts := make(map[nebula.GraphSpaceID][]*meta.PartItem)
	spaces, err := m.ListSpaces()
	if err != nil {
		return nil, err
	}
	for _, space := range spaces {
		spaceDetail, err := m.GetSpace(space.Name)
		if err != nil {
			return nil, err
		}
		if spaceDetail.Properties.ReplicaFactor == 1 {
			continue
		}
		var partIDs []nebula.PartitionID
		for partID := int32(1); partID <= spaceDetail.Properties.PartitionNum; partID++ {
			partIDs = append(partIDs, partID)
		}
		partItems, err := m.ListParts(*space.Id.SpaceID, partIDs)
		if err != nil {
			return nil, err
		}
		spaceParts[*space.Id.SpaceID] = partItems
	}

	return spaceParts, nil
}

func (m *metaClient) GetSpaceLeaderHosts(space []byte) ([]string, error) {
	spaceDetail, err := m.GetSpace(space)
	if err != nil {
		return nil, err
	}
	var partIDs []nebula.PartitionID
	for partID := int32(1); partID <= spaceDetail.Properties.PartitionNum; partID++ {
		partIDs = append(partIDs, partID)
	}

	hosts := sets.NewString()
	partItems, err := m.ListParts(spaceDetail.SpaceID, partIDs)
	if err != nil {
		return nil, err
	}
	for _, partItem := range partItems {
		if partItem.Leader == nil {
			continue
		}
		hosts.Insert(partItem.Leader.Host)
	}
	return hosts.List(), nil
}

func (m *metaClient) GetLeaderCount(leaderHost string) (int, error) {
	spaceItems, err := m.GetSpaceParts()
	if err != nil {
		return 0, err
	}
	count := 0
	for spaceID, partItems := range spaceItems {
		for _, partItem := range partItems {
			if partItem.Leader == nil {
				continue
			}
			if partItem.Leader.Host == leaderHost {
				klog.Infof("space %d partition %d leader still on this node", spaceID, partItem.PartID)
				count++
			}
		}
	}
	return count, nil
}

func (m *metaClient) BalanceLeader(spaceID nebula.GraphSpaceID) error {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_LEADER_BALANCE,
	}
	_, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.RunAdminJob(req.(*meta.AdminJobReq))
		return resp, err
	})
	if err != nil {
		return err
	}
	klog.Infof("space %d balance leader successfully", spaceID)
	return nil
}

func (m *metaClient) IsLeaderBalanced(spaceName []byte) (bool, error) {
	hosts, err := m.ListHosts(meta.ListHostType_ALLOC)
	if err != nil {
		return false, err
	}

	for _, host := range hosts {
		if host.Status != meta.HostStatus_ONLINE {
			continue
		}
		if host.LeaderParts[(string)(spaceName)] == nil {
			return false, nil
		}
	}

	return true, nil
}

func (m *metaClient) runAdminJob(req *meta.AdminJobReq) (int32, error) {
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.RunAdminJob(req.(*meta.AdminJobReq))
		return resp, err
	})
	if err != nil {
		return 0, err
	}
	klog.Info("balance job running now")
	jobID := resp.(*meta.AdminJobResp).GetResult_().GetJobID()
	if jobID == 0 {
		r, err := strconv.Atoi(string(req.Paras[0]))
		if err != nil {
			return 0, err
		}
		jobID = int32(r)
	}
	return jobID, utilerrors.ReconcileErrorf("waiting for balance job %d finished", jobID)
}

func (m *metaClient) BalanceData(spaceID nebula.GraphSpaceID) (int32, error) {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_ZONE_BALANCE,
	}

	return m.runAdminJob(req)
}

func (m *metaClient) BalanceDataInZone(spaceID nebula.GraphSpaceID) (int32, error) {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_DATA_BALANCE,
	}

	return m.runAdminJob(req)
}

func (m *metaClient) RemoveHost(spaceID nebula.GraphSpaceID, hosts []*nebula.HostAddr) (int32, error) {
	paras := make([][]byte, 0)
	for _, endpoint := range hosts {
		// The back quote need here to consistent with the host addr registered in meta
		paras = append(paras, []byte(fmt.Sprintf(`"%s":%d`, endpoint.Host, endpoint.Port)))
	}
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_ZONE_BALANCE,
		Paras:   paras,
	}

	return m.runAdminJob(req)
}

func (m *metaClient) RemoveHostInZone(spaceID nebula.GraphSpaceID, hosts []*nebula.HostAddr) (int32, error) {
	paras := make([][]byte, 0)
	for _, endpoint := range hosts {
		// The back quote need here to consistent with the host addr registered in meta
		paras = append(paras, []byte(fmt.Sprintf(`"%s":%d`, endpoint.Host, endpoint.Port)))
	}
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_DATA_BALANCE,
		Paras:   paras,
	}

	return m.runAdminJob(req)
}

func (m *metaClient) BalanceStatus(jobID int32, spaceID nebula.GraphSpaceID) error {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_SHOW,
		Type:    meta.JobType_STATS,
		Paras:   [][]byte{[]byte(strconv.FormatInt(int64(jobID), 10))},
	}
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.RunAdminJob(req.(*meta.AdminJobReq))
		return resp, err
	})
	if err != nil {
		return err
	}
	jobDesc := resp.(*meta.AdminJobResp).GetResult_().JobDesc
	if len(jobDesc) > 0 {
		if jobDesc[0].Status == meta.JobStatus_FINISHED {
			return nil
		} else if jobDesc[0].Status == meta.JobStatus_FAILED {
			klog.Errorf("space %d balance job %d status failed", spaceID, jobID)
			return ErrJobStatusFailed
		}
	}
	return &utilerrors.ReconcileError{Msg: fmt.Sprintf("balance job still in progress, jobID %d, spaceID %d", jobID, spaceID)}
}

func (m *metaClient) RecoverDataBalanceJob(jobID int32, spaceID nebula.GraphSpaceID) error {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_RECOVER,
		Type:    meta.JobType_ZONE_BALANCE,
		Paras:   [][]byte{[]byte(strconv.FormatInt(int64(jobID), 10))},
	}
	_, err := m.runAdminJob(req)
	return err
}

func (m *metaClient) RecoverInZoneBalanceJob(jobID int32, spaceID nebula.GraphSpaceID) error {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_RECOVER,
		Type:    meta.JobType_DATA_BALANCE,
		Paras:   [][]byte{[]byte(strconv.FormatInt(int64(jobID), 10))},
	}
	_, err := m.runAdminJob(req)
	return err
}

func (m *metaClient) RestoreMeta(hostPairs []*meta.HostPair, files []string) (*meta.RestoreMetaResp, error) {
	byteFiles := make([][]byte, 0, len(files))
	for _, f := range files {
		byteFiles = append(byteFiles, []byte(f))
	}
	req := &meta.RestoreMetaReq{
		Hosts: hostPairs,
		Files: byteFiles,
	}
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.RestoreMeta(req.(*meta.RestoreMetaReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.RestoreMetaResp), nil
}

func (m *metaClient) retryOnError(req interface{}, fn Fn) (interface{}, error) {
	resp, err := fn(req)
	if err != nil {
		return resp, err
	}

	code := getResponseCode(resp)
	if code != nebula.ErrorCode_SUCCEEDED {
		if code == nebula.ErrorCode_E_LEADER_CHANGED {
			leader := getResponseLeader(resp)
			if leader == nil {
				return nil, fmt.Errorf("get changed leader failed")
			}
			newLeader := fmt.Sprintf("%v:%v", leader.Host, leader.Port)
			// update leader info
			if err := m.reconnect(newLeader); err != nil {
				return nil, err
			}
			resp, err := fn(req)
			if err != nil {
				return nil, err
			}
			code := getResponseCode(resp)
			if code != nebula.ErrorCode_SUCCEEDED {
				if code == nebula.ErrorCode_E_EXISTED ||
					code == nebula.ErrorCode_E_NO_HOSTS {
					return resp, nil
				}
				return nil, fmt.Errorf("metad client retry response code %d name %s", code, code.String())
			}
			return resp, nil
		} else if code == nebula.ErrorCode_E_EXISTED ||
			code == nebula.ErrorCode_E_NO_HOSTS {
			return resp, nil
		}
		return nil, fmt.Errorf("metad client response code %d name %s", code, code.String())
	}
	return resp, nil
}

func getResponseCode(resp interface{}) nebula.ErrorCode {
	switch r := resp.(type) {
	case *meta.ExecResp:
		return r.Code
	case *meta.GetSpaceResp:
		return r.Code
	case *meta.ListSpacesResp:
		return r.Code
	case *meta.ListClusterInfoResp:
		return r.Code
	case *meta.ListHostsResp:
		return r.Code
	case *meta.ListPartsResp:
		return r.Code
	case *meta.AdminJobResp:
		return r.Code
	case *meta.RestoreMetaResp:
		return r.Code
	default:
		return nebula.ErrorCode_E_UNKNOWN
	}
}

func getResponseLeader(resp interface{}) *nebula.HostAddr {
	switch r := resp.(type) {
	case *meta.ExecResp:
		return r.Leader
	case *meta.GetSpaceResp:
		return r.Leader
	case *meta.ListSpacesResp:
		return r.Leader
	case *meta.ListClusterInfoResp:
		return r.Leader
	case *meta.ListHostsResp:
		return r.Leader
	case *meta.ListPartsResp:
		return r.Leader
	case *meta.AdminJobResp:
		return r.Leader
	case *meta.RestoreMetaResp:
		return r.Leader
	default:
		return nil
	}
}
