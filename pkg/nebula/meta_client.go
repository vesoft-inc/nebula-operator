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
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"github.com/vesoft-inc/nebula-operator/pkg/util/retry"
)

var ErrNoAvailableMetadEndpoints = errors.New("metadclient: no available endpoints")

var _ MetaInterface = (*metaClient)(nil)

type (
	ExecFn func(req interface{}) (*meta.ExecResp, error)

	MetaInterface interface {
		GetSpace(spaceName string) (*meta.SpaceItem, error)
		ListSpaces() ([]*meta.IdName, error)
		AddHosts(endpoints []*nebula.HostAddr) error
		DropHosts(endpoints []*nebula.HostAddr) error
		ListHosts(hostType meta.ListHostType) ([]*meta.HostItem, error)
		GetPartsAlloc(spaceID nebula.GraphSpaceID) (map[nebula.PartitionID][]*nebula.HostAddr, error)
		ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error)
		GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error)
		GetLeaderCount(leaderHost string) (int, error)
		Balance(req *meta.AdminJobReq) (*meta.AdminJobResp, error)
		BalanceStatus(jobID int32, space []byte) error
		BalanceLeader(space []byte) error
		BalanceData(space []byte) error
		RemoveHost(space []byte, endpoints []*nebula.HostAddr) error
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

func (m *metaClient) reconnect(endpoint string, options ...Option) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.disconnect(); err != nil {
		return err
	}
	if _, err := newMetaConnection(endpoint, options...); err != nil {
		return err
	}
	return nil
}

func (m *metaClient) connect() error {
	log := getLog()
	if err := m.client.Open(); err != nil {
		log.Error(err, "open transport failed")
		return err
	}
	log.Info("metad connection opened", "isOpen", m.client.IsOpen())
	return nil
}

func (m *metaClient) disconnect() error {
	if err := m.client.Close(); err != nil {
		getLog().Error(err, "close transport failed")
	}
	return nil
}

func (m *metaClient) Disconnect() error {
	return m.disconnect()
}

func (m *metaClient) GetSpace(spaceName string) (*meta.SpaceItem, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.GetSpaceReq{SpaceName: []byte(spaceName)}
	resp, err := m.client.GetSpace(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("GetSpace code %d", resp.Code)
	}
	return resp.Item, nil
}

func (m *metaClient) ListSpaces() ([]*meta.IdName, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.ListSpacesReq{}
	resp, err := m.client.ListSpaces(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("ListSpaces code %d", resp.Code)
	}
	return resp.Spaces, nil
}

func (m *metaClient) AddHosts(hosts []*nebula.HostAddr) error {
	req := &meta.AddHostsReq{
		Hosts: hosts,
	}
	return m.retryOnError(req, func(req interface{}) (*meta.ExecResp, error) {
		resp, err := m.client.AddHosts(req.(*meta.AddHostsReq))
		return resp, err
	})
}

func (m *metaClient) DropHosts(hosts []*nebula.HostAddr) error {
	req := &meta.DropHostsReq{
		Hosts: hosts,
	}
	return m.retryOnError(req, func(req interface{}) (*meta.ExecResp, error) {
		resp, err := m.client.DropHosts(req.(*meta.DropHostsReq))
		return resp, err
	})
}

func (m *metaClient) ListHosts(hostType meta.ListHostType) ([]*meta.HostItem, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.ListHostsReq{Type: hostType}
	resp, err := m.client.ListHosts(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("ListHosts code %d", resp.Code)
	}
	return resp.Hosts, nil
}

func (m *metaClient) GetPartsAlloc(spaceID nebula.GraphSpaceID) (map[nebula.PartitionID][]*nebula.HostAddr, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.GetPartsAllocReq{SpaceID: spaceID}
	resp, err := m.client.GetPartsAlloc(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("GetPartsAlloc code %d", resp.Code)
	}
	partMap := make(map[nebula.PartitionID][]*nebula.HostAddr, len(resp.Parts))
	for partID, endpoints := range resp.Parts {
		partMap[partID] = endpoints
	}
	return partMap, nil
}

func (m *metaClient) ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.ListPartsReq{
		SpaceID: spaceID,
		PartIds: partIDs,
	}
	resp, err := m.client.ListParts(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("ListParts code %d", resp.Code)
	}
	return resp.Parts, nil
}

func (m *metaClient) GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error) {
	spaceParts := make(map[nebula.GraphSpaceID][]*meta.PartItem)
	spaces, err := m.ListSpaces()
	if err != nil {
		return nil, err
	}
	for _, space := range spaces {
		spaceDetail, err := m.GetSpace(string(space.Name))
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

func (m *metaClient) BalanceLeader(space []byte) error {
	log := getLog()
	req := &meta.AdminJobReq{
		Op:    meta.AdminJobOp_ADD,
		Cmd:   meta.AdminCmd_LEADER_BALANCE,
		Paras: [][]byte{space},
	}
	resp, err := m.client.RunAdminJob(req)
	if err != nil {
		return err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		if resp.Code == nebula.ErrorCode_E_LEADER_CHANGED {
			log.Info("request leader changed", "host", resp.Leader.Host, "port", resp.Leader.Port)
			leader := fmt.Sprintf("%v:%v", resp.Leader.Host, resp.Leader.Port)
			if err := m.reconnect(leader); err != nil {
				return errors.Errorf("update client failed: %v", err)
			}
			resp, err := m.client.RunAdminJob(req)
			if err != nil {
				return err
			}
			if resp.Code != nebula.ErrorCode_SUCCEEDED {
				return errors.Errorf("retry balance leader code %d", resp.Code)
			}
		} else if resp.Code == nebula.ErrorCode_E_BALANCED {
			log.Info("cluster is balanced")
			return nil
		}
		return errors.Errorf("BalanceLeader code %d", resp.Code)
	}
	log.Info("balance leader successfully")
	return nil
}

func (m *metaClient) balance(space []byte, req *meta.AdminJobReq) error {
	log := getLog()
	log.Info("start balance job")
	resp, err := m.client.RunAdminJob(req)
	if err != nil {
		log.Info("balance failed")
		return err
	}
	log = log.WithValues("BalanceJobID", resp.GetResult_().JobID)
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		if resp.Code == nebula.ErrorCode_E_LEADER_CHANGED {
			log.Info("request leader changed", "host", resp.Leader.Host, "port", resp.Leader.Port)
			leader := fmt.Sprintf("%v:%v", resp.Leader.Host, resp.Leader.Port)
			if err := m.reconnect(leader); err != nil {
				return errors.Errorf("update client failed: %v", err)
			}
			resp, err := m.client.RunAdminJob(req)
			if err != nil {
				return err
			}
			if resp.Code != nebula.ErrorCode_SUCCEEDED {
				return errors.Errorf("retry balance code %d", resp.Code)
			}
			log.Info("balance job running now")
			return m.BalanceStatus(*resp.GetResult_().JobID, space)
		} else if resp.Code == nebula.ErrorCode_E_NO_VALID_HOST {
			return errors.Errorf("the cluster no valid host to balance")
		} else if resp.Code == nebula.ErrorCode_E_BALANCED {
			log.Info("the cluster is balanced")
			return nil
		} else if resp.Code == nebula.ErrorCode_E_NO_HOSTS {
			log.Info("the host is removed")
			return nil
		} else if resp.Code == nebula.ErrorCode_E_BALANCER_RUNNING {
			return errors.Errorf("the cluster balance job is running")
		}
		return errors.Errorf("balance code %d", resp.Code)
	}
	log.Info("balance job running now")
	return m.BalanceStatus(*resp.GetResult_().JobID, space)
}

func (m *metaClient) BalanceData(space []byte) error {
	req := &meta.AdminJobReq{
		Cmd:   meta.AdminCmd_DATA_BALANCE,
		Op:    meta.AdminJobOp_ADD,
		Paras: [][]byte{space},
	}

	return m.balance(space, req)
}

func (m *metaClient) RemoveHost(space []byte, endpoints []*nebula.HostAddr) error {
	paras := make([][]byte, 0)
	for _, endpoint := range endpoints {
		paras = append(paras, []byte(fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)))
	}
	paras = append(paras, space)
	req := &meta.AdminJobReq{
		Cmd:   meta.AdminCmd_DATA_BALANCE,
		Op:    meta.AdminJobOp_ADD,
		Paras: paras,
	}

	return m.balance(space, req)
}

func (m *metaClient) Balance(req *meta.AdminJobReq) (*meta.AdminJobResp, error) {
	return m.client.RunAdminJob(req)
}

func (m *metaClient) BalanceStatus(jobID int32, space []byte) error {
	statusFn := func(ctx context.Context) (bool, error) {
		req := &meta.AdminJobReq{
			Op:    meta.AdminJobOp_SHOW,
			Cmd:   meta.AdminCmd_STATS,
			Paras: [][]byte{[]byte(strconv.FormatInt(int64(jobID), 10)), space},
		}
		resp, err := m.client.RunAdminJob(req)
		if err != nil {
			return false, err
		}
		if resp.Result_.JobDesc[0].Status == meta.JobStatus_FINISHED {
			return true, nil
		}
		return false, nil
	}
	return retry.Until(context.Background(), 5*time.Second, statusFn)
}

func (m *metaClient) retryOnError(req interface{}, fn ExecFn) error {
	resp, err := fn(req)
	if err != nil {
		return err
	}
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		if resp.Code == nebula.ErrorCode_E_LEADER_CHANGED {
			leader := fmt.Sprintf("%v:%v", resp.Leader.Host, resp.Leader.Port)
			if err := m.reconnect(leader); err != nil {
				return errors.Errorf("update client failed: %v", err)
			}
			resp, err := fn(req)
			if err != nil {
				return err
			}
			if resp.Code != nebula.ErrorCode_SUCCEEDED {
				return errors.Errorf("retry response code %d", resp.Code)
			}
		}
		return errors.Errorf("response code %d", resp.Code)
	}
	return nil
}
