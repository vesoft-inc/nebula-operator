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

package nebula

import (
	"context"
	"fmt"

	agentclient "github.com/vesoft-inc/nebula-agent/v3/pkg/client"
	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	rtutil "github.com/vesoft-inc/nebula-operator/pkg/util/restore"
)

type Agent struct {
	agentclient.Client
}

func NewAgent(agentAddr *nebula.HostAddr) (*Agent, error) {
	agentAddr.Port = v1alpha1.DefaultAgentPortGRPC
	cfg := &agentclient.Config{
		Addr: agentAddr,
	}
	c, err := agentclient.New(context.TODO(), cfg)
	if err != nil {
		return nil, err
	}

	a := &Agent{
		Client: c,
	}

	return a, nil
}

type AgentManager struct {
	agents map[string]*Agent
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]*Agent),
	}
}

func (a *AgentManager) GetAgent(agentAddr *nebula.HostAddr) (*Agent, error) {
	if agent, ok := a.agents[agentAddr.Host]; ok {
		return agent, nil
	}

	agent, err := NewAgent(agentAddr)
	if err != nil {
		return nil, fmt.Errorf("create agent %s failed: %v", rtutil.StringifyAddr(agentAddr), err)
	}

	a.agents[agentAddr.Host] = agent
	return agent, nil
}

func (a *AgentManager) Close() {
	for addr, agent := range a.agents {
		agent.Close()
		delete(a.agents, addr)
	}
}
