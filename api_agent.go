// Tencent is pleased to support the open source community by making polaris-go available.
//
// Copyright (C) 2021 Tencent. All rights reserved.
//
// Licensed under the BSD 3-Clause License (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://opensource.org/licenses/BSD-3-Clause
//
// Unless required by applicable law or agreed to in writing, software distributed
// under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied. See the License for the
// specific language governing permissions and limitations under the License.

package polaris

import (
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// GetAgentServiceRequest 精确获取 AI Agent 服务的请求。
type GetAgentServiceRequest api.GetAgentServiceRequest

// EnsureAgentServiceRequest 确保 AI Agent 服务存在。
type EnsureAgentServiceRequest api.EnsureAgentServiceRequest

// MountAgentSkillsRequest 挂载 AI Agent Skill。
type MountAgentSkillsRequest api.MountAgentSkillsRequest

// UnmountAgentSkillsRequest 卸载 AI Agent Skill。
type UnmountAgentSkillsRequest api.UnmountAgentSkillsRequest

// AgentAPI 提供 AI Agent 服务和实例 metadata 能力。
type AgentAPI interface {
	api.SDKOwner
	GetService(req *GetAgentServiceRequest) (*model.AgentService, error)
	EnsureService(req *EnsureAgentServiceRequest) (*model.AgentService, error)
	MountSkills(req *MountAgentSkillsRequest) (*model.AgentService, error)
	UnmountSkills(req *UnmountAgentSkillsRequest) (*model.AgentService, error)
	ValidateInstanceMetadata(metadata map[string]string) error
	Destroy()
}

type agentAPI struct {
	rawAPI api.AgentAPI
}

func (a *agentAPI) SDKContext() api.SDKContext {
	return a.rawAPI.SDKContext()
}

func (a *agentAPI) GetService(req *GetAgentServiceRequest) (*model.AgentService, error) {
	return a.rawAPI.GetService((*api.GetAgentServiceRequest)(req))
}

func (a *agentAPI) EnsureService(req *EnsureAgentServiceRequest) (*model.AgentService, error) {
	return a.rawAPI.EnsureService((*api.EnsureAgentServiceRequest)(req))
}

func (a *agentAPI) MountSkills(req *MountAgentSkillsRequest) (*model.AgentService, error) {
	return a.rawAPI.MountSkills((*api.MountAgentSkillsRequest)(req))
}

func (a *agentAPI) UnmountSkills(req *UnmountAgentSkillsRequest) (*model.AgentService, error) {
	return a.rawAPI.UnmountSkills((*api.UnmountAgentSkillsRequest)(req))
}

func (a *agentAPI) ValidateInstanceMetadata(metadata map[string]string) error {
	return a.rawAPI.ValidateInstanceMetadata(metadata)
}

func (a *agentAPI) Destroy() {
	a.rawAPI.Destroy()
}

// NewAgentAPI 通过默认配置创建 AgentAPI。
func NewAgentAPI() (AgentAPI, error) {
	rawAPI, err := api.NewAgentAPI()
	if err != nil {
		return nil, err
	}
	return &agentAPI{rawAPI: rawAPI}, nil
}

// NewAgentAPIByFile 通过配置文件创建 AgentAPI。
func NewAgentAPIByFile(path string) (AgentAPI, error) {
	rawAPI, err := api.NewAgentAPIByFile(path)
	if err != nil {
		return nil, err
	}
	return &agentAPI{rawAPI: rawAPI}, nil
}

// NewAgentAPIByConfig 通过配置对象创建 AgentAPI。
func NewAgentAPIByConfig(cfg config.Configuration) (AgentAPI, error) {
	rawAPI, err := api.NewAgentAPIByConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &agentAPI{rawAPI: rawAPI}, nil
}

// NewAgentAPIByContext 通过上下文创建 AgentAPI。
func NewAgentAPIByContext(context api.SDKContext) AgentAPI {
	return &agentAPI{rawAPI: api.NewAgentAPIByContext(context)}
}

// NewAgentAPIByDefaultConfigFile 从默认配置文件创建 AgentAPI。
func NewAgentAPIByDefaultConfigFile() (AgentAPI, error) {
	return NewAgentAPIByFile(config.DefaultConfigFile)
}

// NewAgentAPIByAddress 通过地址创建 AgentAPI。
func NewAgentAPIByAddress(address ...string) (AgentAPI, error) {
	rawAPI, err := api.NewAgentAPIByAddress(address...)
	if err != nil {
		return nil, err
	}
	return &agentAPI{rawAPI: rawAPI}, nil
}
