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
	"sync"

	"github.com/pkg/errors"

	"github.com/vesoft-inc/nebula-go/v2/nebula"
	"github.com/vesoft-inc/nebula-go/v2/nebula/storage"
)

var ErrNoAvailableStoragedEndpoints = errors.New("storagedclient: no available endpoints")

var _ StorageInterface = (*storageClient)(nil)

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
	sc, err := newStorageConnection(endpoints[0], options...)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func newStorageConnection(endpoint string, options ...Option) (*storageClient, error) {
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

func (s *storageClient) connect() error {
	log := getLog()
	if err := s.client.Open(); err != nil {
		log.Error(err, "open transport failed")
		return err
	}
	log.Info("storaged connection opened", "isOpen", s.client.IsOpen())
	return nil
}

func (s *storageClient) disconnect() error {
	if err := s.client.Close(); err != nil {
		getLog().Error(err, "close transport failed")
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
	log := getLog()
	log.Info("start TransLeader")
	resp, err := s.client.TransLeader(req)
	if err != nil {
		log.Error(err, "TransLeader failed")
		return err
	}

	if len(resp.Result_.FailedParts) > 0 {
		if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_PART_NOT_FOUND {
			return errors.Errorf("part %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_SPACE_NOT_FOUND {
			return errors.Errorf("space %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_LEADER_CHANGED {
			log.Info("request leader changed", "result", resp.Result_.FailedParts[0].String())
			return nil
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
