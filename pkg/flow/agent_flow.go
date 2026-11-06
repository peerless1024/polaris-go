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

package flow

import "github.com/polarismesh/polaris-go/pkg/model"

// SyncEnsureAgentService 确保 AI Agent 服务存在。
func (e *Engine) SyncEnsureAgentService(req *model.EnsureAgentServiceRequest) (*model.AgentService, error) {
	if e.serviceWriter == nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteUnsupported, nil,
			"serviceWriter.addresses is not configured")
	}
	record, err := e.serviceWriter.EnsureAgentService(req)
	if err != nil {
		return nil, err
	}
	return record.AgentService(), nil
}

// SyncMountAgentSkills 挂载 AI Agent Skill。
func (e *Engine) SyncMountAgentSkills(req *model.MountAgentSkillsRequest) (*model.AgentService, error) {
	if e.serviceWriter == nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteUnsupported, nil,
			"serviceWriter.addresses is not configured")
	}
	skills, err := model.NormalizeAgentSkillValues(req.Skills)
	if err != nil {
		return nil, err
	}
	record, err := e.serviceWriter.MountAgentSkills(req.Namespace, req.Service, skills, req.Replace)
	if err != nil {
		return nil, err
	}
	return record.AgentService(), nil
}

// SyncUnmountAgentSkills 卸载 AI Agent Skill。
func (e *Engine) SyncUnmountAgentSkills(req *model.UnmountAgentSkillsRequest) (*model.AgentService, error) {
	if e.serviceWriter == nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteUnsupported, nil,
			"serviceWriter.addresses is not configured")
	}
	record, err := e.serviceWriter.UnmountAgentSkills(req.Namespace, req.Service, req.SkillIDs)
	if err != nil {
		return nil, err
	}
	return record.AgentService(), nil
}
