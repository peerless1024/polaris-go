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

import "github.com/polarismesh/polaris-go/pkg/model"

// GetAgentServiceRequest 精确获取 AI Agent 服务的请求。
type GetAgentServiceRequest struct {
	model.GetAgentServiceRequest
}

// EnsureAgentServiceRequest 确保 AI Agent 服务存在。
type EnsureAgentServiceRequest struct {
	model.EnsureAgentServiceRequest
}

// MountAgentSkillsRequest 挂载 AI Agent Skill。
type MountAgentSkillsRequest struct {
	model.MountAgentSkillsRequest
}

// UnmountAgentSkillsRequest 卸载 AI Agent Skill。
type UnmountAgentSkillsRequest struct {
	model.UnmountAgentSkillsRequest
}

// AgentAPI 提供 AI Agent 服务和实例 metadata 能力。
// 同一 SDKContext 的同服务写入串行执行；跨 SDKContext/进程写入仍需要单 owner 协调。
type AgentAPI interface {
	SDKOwner
	// GetService 精确获取 AI Agent 服务。
	GetService(req *GetAgentServiceRequest) (*model.AgentService, error)
	// EnsureService 确保 AI Agent 服务存在；服务已存在时不更新 metadata/Skill。
	EnsureService(req *EnsureAgentServiceRequest) (*model.AgentService, error)
	// MountSkills 挂载服务级 Skill。
	MountSkills(req *MountAgentSkillsRequest) (*model.AgentService, error)
	// UnmountSkills 卸载服务级 Skill。
	UnmountSkills(req *UnmountAgentSkillsRequest) (*model.AgentService, error)
	// ValidateInstanceMetadata 校验 Agent 实例 metadata。
	ValidateInstanceMetadata(metadata map[string]string) error
	// Destroy 销毁 API。
	Destroy()
}

var (
	// NewAgentAPI 通过默认配置创建 AgentAPI。
	NewAgentAPI = newAgentAPI
	// NewAgentAPIByFile 通过配置文件创建 AgentAPI。
	NewAgentAPIByFile = newAgentAPIByFile
	// NewAgentAPIByConfig 通过配置对象创建 AgentAPI。
	NewAgentAPIByConfig = newAgentAPIByConfig
	// NewAgentAPIByContext 通过上下文创建 AgentAPI。
	NewAgentAPIByContext = newAgentAPIByContext
	// NewAgentAPIByDefaultConfigFile 从默认配置文件创建 AgentAPI。
	NewAgentAPIByDefaultConfigFile = newAgentAPIByDefaultConfigFile
	// NewAgentAPIByAddress 通过地址创建 AgentAPI。
	NewAgentAPIByAddress = newAgentAPIByAddress
)
