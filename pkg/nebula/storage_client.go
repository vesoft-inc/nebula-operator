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
	"github.com/vesoft-inc/nebula-go/nebula/storage"
)

var ErrNoAvailableStoragedEndpoints = errors.New("storagedclient: no available endpoints")

var _ StorageInterface = &storageClient{}

type StorageInterface interface {
	TransLeader(spaceID nebula.GraphSpaceID, partID nebula.PartitionID, newLeader *nebula.HostAddr) error
	RemovePart(spaceID nebula.GraphSpaceID, partID nebula.PartitionID) error
	GetLeaderParts() (map[nebula.GraphSpaceID][]nebula.PartitionID, error)
	Disconnect() error
}

type storageClient struct {
	mutex  sync.Mutex
	client *storage.StorageAdminServiceClient
}

func NewStorageClient(endpoints []string, options ...Option) (StorageInterface, error) {
	if len(endpoints) == 0 {
		return nil, ErrNoAvailableMetadEndpoints
	}
	sc, err := newStoragedClient(endpoints[0], options...)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func newStoragedClient(endpoint string, options ...Option) (*storageClient, error) {
	transport, pf, err := buildClientTransport(endpoint, options...)
	if err != nil {
		return nil, err
	}
	storageServiceClient := storage.NewStorageAdminServiceClientFactory(transport, pf)
	sc := &storageClient{client: storageServiceClient}
	if err := sc.connect(); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *storageClient) updateClient(endpoint string, options ...Option) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.disconnect(); err != nil {
		return err
	}
	if _, err := newStoragedClient(endpoint, options...); err != nil {
		return err
	}
	return nil
}

func (s *storageClient) connect() error {
	if err := s.client.Transport.Open(); err != nil {
		return err
	}
	klog.Infof("storaged connection opened %v", s.client.Transport.IsOpen())
	return nil
}

func (s *storageClient) disconnect() error {
	if err := s.client.Close(); err != nil {
		klog.Errorf("Fail to close transport, error: %s", err.Error())
	}
	return nil
}

func (s *storageClient) Disconnect() error {
	return s.disconnect()
}

func (s *storageClient) TransLeader(spaceID nebula.GraphSpaceID, partID nebula.PartitionID, newLeader *nebula.HostAddr) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	req := &storage.TransLeaderReq{
		SpaceID:    spaceID,
		PartID:     partID,
		NewLeader_: newLeader,
	}
	resp, err := s.client.TransLeader(req)
	if err != nil {
		return err
	}
	if len(resp.Result_.FailedParts) > 0 {
		if resp.Result_.FailedParts[0].Code == storage.ErrorCode_E_PART_NOT_FOUND {
			return errors.Errorf("part %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == storage.ErrorCode_E_SPACE_NOT_FOUND {
			return errors.Errorf("space %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == storage.ErrorCode_E_TRANSFER_LEADER_FAILED {
			klog.Infof("request should send to %v:%v", resp.Result_.FailedParts[0].Leader.Host, resp.Result_.FailedParts[0].Leader.Port)
			leader := fmt.Sprintf("%s:%v", resp.Result_.FailedParts[0].Leader.Host, resp.Result_.FailedParts[0].Leader.Port)
			if err := s.updateClient(leader); err != nil {
				return err
			}
			resp, err := s.client.TransLeader(req)
			if err != nil {
				return err
			}
			if len(resp.Result_.FailedParts) > 0 {
				return errors.Errorf("Trans leader failed")
			}
			return nil
		} else if resp.Result_.FailedParts[0].Code == storage.ErrorCode_E_LEADER_CHANGED {
			return errors.Errorf("leader changed, please request to leader")
		} else {
			return errors.Errorf("TransLeader space %d part %d code %d", spaceID, partID, resp.Result_.FailedParts[0].Code)
		}
	}
	return nil
}

func (s *storageClient) RemovePart(spaceID nebula.GraphSpaceID, partID nebula.PartitionID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	req := &storage.RemovePartReq{
		SpaceID: spaceID,
		PartID:  partID,
	}
	resp, err := s.client.RemovePart(req)
	if err != nil {
		return err
	}
	if len(resp.Result_.FailedParts) > 0 {
		return errors.Errorf("RemovePart code %d", resp.Result_.FailedParts[0].Code)
	}
	return nil
}

func (s *storageClient) GetLeaderParts() (map[nebula.GraphSpaceID][]nebula.PartitionID, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	req := &storage.GetLeaderReq{}
	resp, err := s.client.GetLeaderParts(req)
	if err != nil {
		return nil, err
	}
	if len(resp.Result_.FailedParts) > 0 {
		return nil, errors.Errorf("GetLeaderParts code %d", resp.Result_.FailedParts[0].Code)
	}
	return resp.GetLeaderParts(), nil
}
