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
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/storage"
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
	client *storage.StorageAdminServiceClient
}

func NewStorageClient(endpoints []string, options ...Option) (StorageInterface, error) {
	if len(endpoints) == 0 {
		return nil, ErrNoAvailableStoragedEndpoints
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
	if err := s.client.Open(); err != nil {
		return err
	}
	if !s.client.IsOpen() {
		return errors.Errorf("transport is not open")
	}
	return nil
}

func (s *storageClient) Disconnect() error {
	return s.client.Close()
}

func (s *storageClient) TransLeader(spaceID nebula.GraphSpaceID, partID nebula.PartitionID, newLeader *nebula.HostAddr) error {
	req := &storage.TransLeaderReq{
		SpaceID:    spaceID,
		PartID:     partID,
		NewLeader_: newLeader,
	}
	klog.Infof("start transfer leader spaceID %d partitionID %d", spaceID, partID)
	resp, err := s.client.TransLeader(req)
	if err != nil {
		klog.Errorf("TransLeader failed: %v", err)
		return err
	}

	if len(resp.Result_.FailedParts) > 0 {
		if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_PART_NOT_FOUND {
			return errors.Errorf("partition %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_SPACE_NOT_FOUND {
			return errors.Errorf("space %d not found", partID)
		} else if resp.Result_.FailedParts[0].Code == nebula.ErrorCode_E_LEADER_CHANGED {
			klog.Infof("request leader changed, result: %v", resp.Result_.FailedParts[0].String())
			return nil
		} else {
			return errors.Errorf("TransLeader space %d partition %d code %d", spaceID, partID, resp.Result_.FailedParts[0].Code)
		}
	}
	return nil
}

func (s *storageClient) RemovePart(spaceID nebula.GraphSpaceID, partID nebula.PartitionID) error {
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
	req := &storage.GetLeaderReq{}
	resp, err := s.client.GetLeaderParts(req)
	if err != nil {
		return nil, err
	}
	return resp.GetLeaderParts(), nil
}
