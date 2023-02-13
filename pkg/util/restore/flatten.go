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

package restore

import (
	"fmt"
	"strconv"

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
)

// PartInfo save a unique part's information for FlattenBackupMeta
type PartInfo struct {
	SpaceID        string
	PartID         string
	HostAddr       *nebula.HostAddr
	DataPath       string
	CheckpointPath string
	CommitLogId    int64
	LastLogId      int64
}

// FlattenBackupMeta flatten backup meta to a map for convenience
// because of (spaceId + partId) can specify a unique part
func FlattenBackupMeta(backup *meta.BackupMeta) map[string]*PartInfo {
	backupMap := make(map[string]*PartInfo)
	for sid, sb := range backup.GetSpaceBackups() {
		for _, hb := range sb.GetHostBackups() {
			for _, ck := range hb.GetCheckpoints() {
				for pid, part := range ck.GetParts() {
					key := GenPartKey(strconv.Itoa(int(sid)), strconv.Itoa(int(pid)))
					backupMap[key] = &PartInfo{
						SpaceID:        strconv.Itoa(int(sid)),
						PartID:         strconv.Itoa(int(pid)),
						HostAddr:       hb.GetHost(),
						DataPath:       string(ck.DataPath),
						CheckpointPath: string(part.CheckpointPath),
						CommitLogId:    part.CommitLogID,
						LastLogId:      part.LogID,
					}
				}
			}
		}
	}
	return backupMap
}

// FlattenRestoreMeta flatten meta.RestoreMetaResp to a map for convenience
// because of (spaceId + partId) can specify a unique part
func FlattenRestoreMeta(resp *meta.RestoreMetaResp) map[string][]*nebula.HostAddr {
	hostMap := make(map[string][]*nebula.HostAddr)
	for sid, parts := range resp.GetPartHosts() {
		for _, part := range parts {
			key := GenPartKey(strconv.Itoa(int(sid)), strconv.Itoa(int(part.PartID)))
			hostMap[key] = part.GetHosts()
		}
	}
	return hostMap
}

// GenPartKey generate a unique key for part
func GenPartKey(spaceId, partId string) string {
	return fmt.Sprintf("%s-%s", spaceId, partId)
}

// GenDataPathKey generate a unique key for data path
func GenDataPathKey(host *nebula.HostAddr, partKey string) string {
	return fmt.Sprintf("%s-%s", StringifyAddr(host), partKey)
}
