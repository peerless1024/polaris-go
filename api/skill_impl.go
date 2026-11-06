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

type skillAPI struct {
	context SDKContext
}

// GetSkill 获取单个 published Skill。
func (s *skillAPI) GetSkill(req *GetSkillRequest) (*model.SkillResource, error) {
	if err := checkAvailable(s); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"GetSkillRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return s.context.GetEngine().SyncGetSkill(&req.GetSkillRequest)
}

// GetSkillList 获取 published Skill 列表。
func (s *skillAPI) GetSkillList(req *GetSkillListRequest) ([]*model.SkillResource, error) {
	if err := checkAvailable(s); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"GetSkillListRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return s.context.GetEngine().SyncGetSkillList(&req.GetSkillListRequest)
}

// DownloadSkill 下载 Skill。
func (s *skillAPI) DownloadSkill(req *SkillDownloadRequest) (*model.SkillPackage, error) {
	if err := checkAvailable(s); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"SkillDownloadRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	downloadReq := req.SkillDownloadRequest
	if downloadReq.Format == "" {
		downloadReq.Format = model.SkillFormatMarkdown
	}
	return s.context.GetEngine().SyncDownloadSkill(&downloadReq)
}

// Install 下载并安装 Skill。
func (s *skillAPI) Install(req *SkillInstallRequest) (*model.InstalledSkill, error) {
	if err := checkAvailable(s); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, model.NewSDKError(model.ErrCodeAPIInvalidArgument, nil,
			"SkillInstallRequest can not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return s.context.GetEngine().SyncInstallSkill(&req.SkillInstallRequest)
}

// SDKContext 获取 SDK 上下文。
func (s *skillAPI) SDKContext() SDKContext {
	return s.context
}

// Destroy 销毁 API。
func (s *skillAPI) Destroy() {
	if s.context != nil {
		s.context.Destroy()
	}
}

func newSkillAPI() (SkillAPI, error) {
	return NewSkillAPIByConfig(config.NewDefaultConfigurationWithDomain())
}

func newSkillAPIByFile(path string) (SkillAPI, error) {
	context, err := InitContextByFile(path)
	if err != nil {
		return nil, err
	}
	return &skillAPI{context: context}, nil
}

func newSkillAPIByConfig(cfg config.Configuration) (SkillAPI, error) {
	context, err := InitContextByConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &skillAPI{context: context}, nil
}

func newSkillAPIByContext(context SDKContext) SkillAPI {
	return &skillAPI{context: context}
}

func newSkillAPIByDefaultConfigFile() (SkillAPI, error) {
	return NewSkillAPIByFile(config.DefaultConfigFile)
}

func newSkillAPIByAddress(address ...string) (SkillAPI, error) {
	return newSkillAPIByConfig(config.NewDefaultConfiguration(address))
}
