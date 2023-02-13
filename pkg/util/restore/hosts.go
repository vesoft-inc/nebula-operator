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

	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
)

type NebulaHosts struct {
	hosts map[string][]*meta.ServiceInfo // ip -> (agent, [storaged, metad, graphd, listener])
}

type HostDir struct {
	Host string // ip
	Dir  string // nebula root dir
}

func (h *NebulaHosts) LoadFrom(resp *meta.ListClusterInfoResp) error {
	if resp.Code != nebula.ErrorCode_SUCCEEDED {
		return fmt.Errorf("response is not successful, code is %s", resp.GetCode().String())
	}

	// only load metad、garphd、storaged、agent role
	h.hosts = make(map[string][]*meta.ServiceInfo)
	for host, services := range resp.GetHostServices() {
		for _, s := range services {
			switch s.GetRole() {
			case meta.HostRole_GRAPH, meta.HostRole_META, meta.HostRole_STORAGE, meta.HostRole_AGENT:
				h.hosts[host] = append(h.hosts[host], s)
			}

		}
	}

	// check only one agent in each host
	for _, services := range h.hosts {
		var agentAddr *nebula.HostAddr
		for _, s := range services {
			if s.GetRole() == meta.HostRole_AGENT {
				if agentAddr == nil {
					agentAddr = s.GetAddr()
				} else {
					return fmt.Errorf("there are more than one agent in host %s: %s, %s", s.GetAddr().GetHost(),
						StringifyAddr(agentAddr), StringifyAddr(s.GetAddr()))
				}
			}
		}
	}

	return nil
}

func (h *NebulaHosts) GetMetas() []*meta.ServiceInfo {
	var sl []*meta.ServiceInfo
	for _, services := range h.hosts {
		for _, s := range services {
			if s.Role == meta.HostRole_META {
				sl = append(sl, s)
			}
		}
	}

	return sl
}

func (h *NebulaHosts) GetStorages() []*meta.ServiceInfo {
	var sl []*meta.ServiceInfo
	for _, services := range h.hosts {
		for _, s := range services {
			if s.Role == meta.HostRole_STORAGE {
				sl = append(sl, s)
			}
		}
	}

	return sl
}
