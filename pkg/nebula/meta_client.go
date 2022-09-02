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
	"sync"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

var ErrNoAvailableMetadEndpoints = errors.New("metadclient: no available endpoints")

var _ MetaInterface = (*metaClient)(nil)

type (
	Fn func(req interface{}) (interface{}, error)

	MetaInterface interface {
		GetSpace(spaceName []byte) (*meta.SpaceItem, error)
		ListSpaces() ([]*meta.IdName, error)
		AddHosts(endpoints []*nebula.HostAddr) error
		DropHosts(endpoints []*nebula.HostAddr) error
		ListHosts(hostType meta.ListHostType) ([]*meta.HostItem, error)
		ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error)
		GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error)
		GetSpaceLeaderHosts(space []byte) ([]string, error)
		GetLeaderCount(leaderHost string) (int, error)
		BalanceStatus(jobID int32, spaceID nebula.GraphSpaceID) error
		BalanceLeader(spaceID nebula.GraphSpaceID) error
		BalanceData(spaceID nebula.GraphSpaceID) (int32, error)
		RemoveHost(spaceID nebula.GraphSpaceID, endpoints []*nebula.HostAddr) (int32, error)
		Disconnect() error
	}

	metaClient struct {
		mutex  sync.Mutex
		client *meta.MetaServiceClient
	}
)

func NewMetaClient(endpoints []string, options ...Option) (MetaInterface, error) {
	if len(endpoints) == 0 {
		return nil, ErrNoAvailableMetadEndpoints
	}
	mc, err := newMetaConnection(endpoints[0], options...)
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
	mc := &metaClient{client: metaServiceClient}
	if err := mc.connect(); err != nil {
		return nil, err
	}
	return mc, nil
}

func (m *metaClient) reconnect(endpoint string) error {
	if err := m.client.Close(); err != nil {
		return err
	}

	transport, pf, err := buildClientTransport(endpoint)
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
	m.mutex.Lock()
	defer m.mutex.Unlock()
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

func (m *metaClient) ListSpaces() ([]*meta.IdName, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	resp, err := m.retryOnError(meta.NewListSpacesReq(), func(req interface{}) (interface{}, error) {
		resp, err := m.client.ListSpaces(req.(*meta.ListSpacesReq))
		return resp, err
	})
	if err != nil {
		return nil, err
	}
	return resp.(*meta.ListSpacesResp).Spaces, nil
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
	m.mutex.Lock()
	defer m.mutex.Unlock()
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
	m.mutex.Lock()
	defer m.mutex.Unlock()
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
	log := getLog()
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
				log.Info("space's partition still distribute this node",
					"space", spaceID, "partition", partItem.PartID)
				count++
			}
		}
	}
	return count, nil
}

func (m *metaClient) BalanceLeader(spaceID nebula.GraphSpaceID) error {
	log := getLog().WithValues("SpaceID", spaceID)
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
	log.Info("balance leader successfully")
	return nil
}

func (m *metaClient) balance(req *meta.AdminJobReq) (int32, error) {
	log := getLog()
	log.Info("start balance job")
	resp, err := m.retryOnError(req, func(req interface{}) (interface{}, error) {
		resp, err := m.client.RunAdminJob(req.(*meta.AdminJobReq))
		return resp, err
	})
	if err != nil {
		return 0, err
	}
	log.Info("balance job running now")
	jobID := resp.(*meta.AdminJobResp).GetResult_().GetJobID()
	return jobID, utilerrors.ReconcileErrorf("waiting for balance job %d finished", jobID)
}

func (m *metaClient) BalanceData(spaceID nebula.GraphSpaceID) (int32, error) {
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_ZONE_BALANCE,
	}

	return m.balance(req)
}

func (m *metaClient) RemoveHost(spaceID nebula.GraphSpaceID, endpoints []*nebula.HostAddr) (int32, error) {
	paras := make([][]byte, 0)
	for _, endpoint := range endpoints {
		// The back quote need here to consistent with the host addr registered in meta
		paras = append(paras, []byte(fmt.Sprintf(`"%s":%d`, endpoint.Host, endpoint.Port)))
	}
	req := &meta.AdminJobReq{
		SpaceID: spaceID,
		Op:      meta.JobOp_ADD,
		Type:    meta.JobType_ZONE_BALANCE,
		Paras:   paras,
	}

	return m.balance(req)
}

func (m *metaClient) BalanceStatus(jobID int32, spaceID nebula.GraphSpaceID) error {
	log := getLog().WithValues("JobID", jobID, "SpaceID", spaceID)
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
		}
	}
	log.Info("Balance job in progress")
	return &utilerrors.ReconcileError{Msg: fmt.Sprintf("Balance job %d still in progress", jobID)}
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
				return nil, fmt.Errorf("retry response code %d", code)
			} else if code == nebula.ErrorCode_E_EXISTED {
				return resp, nil
			}
			return resp, nil
		} else if code == nebula.ErrorCode_E_EXISTED {
			return resp, nil
		} else if code == nebula.ErrorCode_E_NO_HOSTS {
			return resp, nil
		}
		return nil, fmt.Errorf("response code %d", code)
	}
	return resp, nil
}

func getResponseCode(resp interface{}) nebula.ErrorCode {
	if r, ok := resp.(*meta.ExecResp); ok {
		return r.Code
	} else if r, ok := resp.(*meta.GetSpaceResp); ok {
		return r.Code
	} else if r, ok := resp.(*meta.ListSpacesResp); ok {
		return r.Code
	} else if r, ok := resp.(*meta.ListHostsResp); ok {
		return r.Code
	} else if r, ok := resp.(*meta.ListPartsResp); ok {
		return r.Code
	} else if r, ok := resp.(*meta.AdminJobResp); ok {
		return r.Code
	}
	return nebula.ErrorCode_E_UNKNOWN
}

func getResponseLeader(resp interface{}) *nebula.HostAddr {
	if r, ok := resp.(*meta.ExecResp); ok {
		return r.Leader
	} else if r, ok := resp.(*meta.GetSpaceResp); ok {
		return r.Leader
	} else if r, ok := resp.(*meta.ListSpacesResp); ok {
		return r.Leader
	} else if r, ok := resp.(*meta.ListHostsResp); ok {
		return r.Leader
	} else if r, ok := resp.(*meta.ListPartsResp); ok {
		return r.Leader
	} else if r, ok := resp.(*meta.AdminJobResp); ok {
		return r.Leader
	}
	return nil
}
