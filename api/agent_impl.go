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

package api

import (
	"github.com/polarismesh/polaris-go/pkg/config"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type agentAPI struct {
	context SDKContext
}

// GetService 精确获取 AI Agent 服务。
func (a *agentAPI) GetService(req *GetAgentServiceRequest) (*model.AgentService, error) {
	if err := checkAvailable(a); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"GetAgentServiceRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return a.context.GetEngine().SyncGetAgentService(&req.GetAgentServiceRequest)
}

// EnsureService 确保 AI Agent 服务存在。
func (a *agentAPI) EnsureService(req *EnsureAgentServiceRequest) (*model.AgentService, error) {
	if err := checkAvailable(a); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"EnsureAgentServiceRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return a.context.GetEngine().SyncEnsureAgentService(&req.EnsureAgentServiceRequest)
}

// MountSkills 挂载服务级 Skill。
func (a *agentAPI) MountSkills(req *MountAgentSkillsRequest) (*model.AgentService, error) {
	if err := checkAvailable(a); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"MountAgentSkillsRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return a.context.GetEngine().SyncMountAgentSkills(&req.MountAgentSkillsRequest)
}

// UnmountSkills 卸载服务级 Skill。
func (a *agentAPI) UnmountSkills(req *UnmountAgentSkillsRequest) (*model.AgentService, error) {
	if err := checkAvailable(a); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"UnmountAgentSkillsRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return a.context.GetEngine().SyncUnmountAgentSkills(&req.UnmountAgentSkillsRequest)
}

// ValidateInstanceMetadata 校验 Agent 实例 metadata。
func (a *agentAPI) ValidateInstanceMetadata(metadata map[string]string) error {
	if err := checkAvailable(a); err != nil {
		return err
	}
	return model.ValidateAgentInstanceMetadata(metadata)
}

// SDKContext 获取 SDK 上下文。
func (a *agentAPI) SDKContext() SDKContext {
	return a.context
}

// Destroy 销毁 API。
func (a *agentAPI) Destroy() {
	if a.context != nil {
		a.context.Destroy()
	}
}

func newAgentAPI() (AgentAPI, error) {
	return NewAgentAPIByConfig(config.NewDefaultConfigurationWithDomain())
}

func newAgentAPIByFile(path string) (AgentAPI, error) {
	context, err := InitContextByFile(path)
	if err != nil {
		return nil, err
	}
	return &agentAPI{context: context}, nil
}

func newAgentAPIByConfig(cfg config.Configuration) (AgentAPI, error) {
	context, err := InitContextByConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &agentAPI{context: context}, nil
}

func newAgentAPIByContext(context SDKContext) AgentAPI {
	return &agentAPI{context: context}
}

func newAgentAPIByDefaultConfigFile() (AgentAPI, error) {
	return NewAgentAPIByFile(config.DefaultConfigFile)
}

func newAgentAPIByAddress(address ...string) (AgentAPI, error) {
	return newAgentAPIByConfig(config.NewDefaultConfiguration(address))
}
