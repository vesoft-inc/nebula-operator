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
	"sync"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-go/nebula"
	"github.com/vesoft-inc/nebula-go/nebula/meta"
	utilerrors "github.com/vesoft-inc/nebula-operator/pkg/util/errors"
)

var ErrNoAvailableMetadEndpoints = errors.New("metadclient: no available endpoints")

var _ MetaInterface = &metaClient{}

type MetaInterface interface {
	GetSpace(spaceName string) (*meta.SpaceItem, error)
	ListSpaces() ([]*meta.IdName, error)
	ListHosts() ([]*meta.HostItem, error)
	GetPartsAlloc(spaceID nebula.GraphSpaceID) (map[nebula.PartitionID][]*nebula.HostAddr, error)
	ListParts(spaceID nebula.GraphSpaceID, partIDs []nebula.PartitionID) ([]*meta.PartItem, error)
	GetSpaceParts() (map[nebula.GraphSpaceID][]*meta.PartItem, error)
	GetLeaderCount(leaderHost string) (int, error)
	Balance(req *meta.BalanceReq) (*meta.BalanceResp, error)
	BalanceStatus(id int64) error
	BalanceLeader() error
	BalanceData() (int64, error)
	RemoveHost(endpoints []*nebula.HostAddr) error
	BalanceStop(stop bool) error
	Disconnect() error
}

type metaClient struct {
	mutex  sync.Mutex
	client *meta.MetaServiceClient
}

func NewMetaClient(endpoints []string, options ...Option) (MetaInterface, error) {
	if len(endpoints) == 0 {
		return nil, ErrNoAvailableMetadEndpoints
	}
	mc, err := newMetadClient(endpoints[0], options...)
	if err != nil {
		return nil, err
	}
	return mc, nil
}

func newMetadClient(endpoint string, options ...Option) (*metaClient, error) {
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

func (m *metaClient) updateClient(endpoint string, options ...Option) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.disconnect(); err != nil {
		return err
	}
	if _, err := newMetadClient(endpoint, options...); err != nil {
		return err
	}
	return nil
}

func (m *metaClient) connect() error {
	if err := m.client.Transport.Open(); err != nil {
		return err
	}
	klog.Infof("metad connection opened %v", m.client.Transport.IsOpen())
	return nil
}

func (m *metaClient) disconnect() error {
	if err := m.client.Close(); err != nil {
		klog.Errorf("Fail to close transport, error: %s", err.Error())
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
	if resp.Code != meta.ErrorCode_SUCCEEDED {
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
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		return nil, errors.Errorf("ListSpaces code %d", resp.Code)
	}
	return resp.Spaces, nil
}

func (m *metaClient) ListHosts() ([]*meta.HostItem, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	req := &meta.ListHostsReq{}
	resp, err := m.client.ListHosts(req)
	if err != nil {
		return nil, err
	}
	if resp.Code != meta.ErrorCode_SUCCEEDED {
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
	if resp.Code != meta.ErrorCode_SUCCEEDED {
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
	if resp.Code != meta.ErrorCode_SUCCEEDED {
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
		var partIDs []nebula.PartitionID
		for partID := int32(1); partID <= spaceDetail.Properties.PartitionNum; partID++ {
			partIDs = append(partIDs, nebula.PartitionID(partID))
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
				klog.Infof("space %d partition %d still distribute this node", spaceID, partItem.PartID)
				count++
			}
		}
	}
	return count, nil
}

func (m *metaClient) BalanceLeader() error {
	req := &meta.LeaderBalanceReq{}
	resp, err := m.client.LeaderBalance(req)
	if err != nil {
		return err
	}
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		return errors.Errorf("BalanceLeader code %d", resp.Code)
	}
	return nil
}

func (m *metaClient) BalanceData() (int64, error) {
	req := &meta.BalanceReq{}
	resp, err := m.client.Balance(req)
	if err != nil {
		return 0, err
	}
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		if resp.Code == meta.ErrorCode_E_LEADER_CHANGED {
			klog.Infof("request should send to %v:%v", resp.Leader.Host, resp.Leader.Port)
			leader := fmt.Sprintf("%v:%v", resp.Leader.Host, resp.Leader.Port)
			if err := m.updateClient(leader); err != nil {
				return 0, errors.Errorf("update client failed: %v", err)
			}
			resp, err := m.Balance(&meta.BalanceReq{})
			if err != nil {
				return 0, err
			}
			if resp.Code != meta.ErrorCode_SUCCEEDED {
				return 0, errors.Errorf("Retry BalanceData code %d", resp.Code)
			}
			return resp.Id, nil
		} else if resp.Code == meta.ErrorCode_E_NO_VALID_HOST {
			return 0, errors.Errorf("The cluster no valid host to balance")
		} else if resp.Code == meta.ErrorCode_E_BALANCED {
			klog.Info("The cluster is balanced")
			return resp.Id, nil
		}
		return 0, errors.Errorf("BalanceData code %d", resp.Code)
	}
	return resp.Id, nil
}

func (m *metaClient) RemoveHost(endpoints []*nebula.HostAddr) error {
	req := &meta.BalanceReq{
		HostDel: endpoints,
	}
	klog.Infof("balance data remove hosts: %v", endpoints)
	resp, err := m.Balance(req)
	if err != nil {
		return err
	}
	klog.Infof("balance Id: %v", resp.Id)
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		if resp.Code == meta.ErrorCode_E_LEADER_CHANGED {
			klog.Infof("request should send to %v:%v", resp.Leader.Host, resp.Leader.Port)
			leader := fmt.Sprintf("%v:%v", resp.Leader.Host, resp.Leader.Port)
			if err := m.updateClient(leader); err != nil {
				return errors.Errorf("update client failed: %v", err)
			}
			_, err := m.Balance(&meta.BalanceReq{})
			if err != nil {
				return err
			}
			req := &meta.BalanceReq{
				HostDel: endpoints,
			}
			resp, err = m.Balance(req)
			if err != nil {
				return err
			}
			if resp.Code != meta.ErrorCode_SUCCEEDED {
				return errors.Errorf("Retry RemoveHost code %d", resp.Code)
			}
			klog.Infof("Balance plan %d running now", resp.Id)
			if err := m.BalanceStatus(resp.Id); err != nil {
				return err
			}
			return nil
		} else if resp.Code == meta.ErrorCode_E_BALANCED {
			klog.Info("The cluster is balanced")
			return nil
		} else if resp.Code == meta.ErrorCode_E_NO_VALID_HOST {
			return errors.Errorf("The cluster no valid host to balance")
		}
		return errors.Errorf("RemoveHost code %d", resp.Code)
	}
	klog.Infof("Balance plan %d running now", resp.Id)
	if err := m.BalanceStatus(resp.Id); err != nil {
		return err
	}
	return nil
}

func (m *metaClient) Balance(req *meta.BalanceReq) (*meta.BalanceResp, error) {
	return m.client.Balance(req)
}

func (m *metaClient) BalanceStatus(id int64) error {
	resp, err := m.Balance(&meta.BalanceReq{Id: &id})
	if err != nil {
		return err
	}
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		return errors.Errorf("BalanceStatus code %d", resp.Code)
	}

	klog.Infof("Get balance plan %d status", id)
	done := 0
	for _, task := range resp.Tasks {
		if task.Result_ == meta.TaskResult__SUCCEEDED {
			done++
		} else if task.Result_ == meta.TaskResult__IN_PROGRESS {
			continue
		} else if task.Result_ == meta.TaskResult__FAILED || task.Result_ == meta.TaskResult__INVALID {
			return errors.Errorf("task %s status %s", task.Id, task.Result_.String())
		}
	}
	if done == len(resp.Tasks) {
		klog.Infof("Balance plan %d done!", id)
		return nil
	}
	return &utilerrors.ReconcileError{Msg: fmt.Sprintf("Balance plan %d still in progress", id)}
}

func (m *metaClient) BalanceStop(stop bool) error {
	req := &meta.BalanceReq{
		Stop: &stop,
	}
	resp, err := m.client.Balance(req)
	if err != nil {
		return err
	}
	if resp.Code != meta.ErrorCode_SUCCEEDED {
		if resp.Code == meta.ErrorCode_E_NO_RUNNING_BALANCE_PLAN {
			return errors.Errorf("BalanceStop no running balance plan")
		}
		return errors.Errorf("BalanceStop code %d", resp.Code)
	}
	return nil
}
