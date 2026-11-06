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

package model

import (
	"encoding/json"
	"fmt"
)

// AgentInterface 是 AgentCard 中声明的协议无关 endpoint。
type AgentInterface struct {
	URL             string
	Transport       string
	ProtocolVersion string
	Tenant          string
}

// AgentCard 是 polaris-go 使用的最小 AgentCard 视图。
type AgentCard struct {
	Name                string
	Description         string
	Version             string
	URL                 string
	SupportedInterfaces []AgentInterface
	Skills              []AgentSkill
	legacyURLOnly       bool
}

// PrimaryInterface 返回首选接口，兼容仅设置顶层 URL 的旧卡片。
func (c *AgentCard) PrimaryInterface() (AgentInterface, bool) {
	if c == nil {
		return AgentInterface{}, false
	}
	if len(c.SupportedInterfaces) > 0 {
		return c.SupportedInterfaces[0], true
	}
	if c.URL != "" {
		return AgentInterface{URL: c.URL}, true
	}
	return AgentInterface{}, false
}

type agentCardJSON struct {
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Version             string               `json:"version"`
	URL                 string               `json:"url"`
	PreferredTransport  string               `json:"preferredTransport"`
	SupportedInterfaces []agentInterfaceJSON `json:"supportedInterfaces"`
	Skills              []agentSkillJSON     `json:"skills"`
}

type agentInterfaceJSON struct {
	URL             string `json:"url"`
	Transport       string `json:"transport"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
	Tenant          string `json:"tenant"`
}

type agentSkillJSON struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
	Version     string   `json:"version"`
}

// ParseAgentCard 解析 AgentCard JSON，并保留调用方传入的原始 JSON 语义。
func ParseAgentCard(cardJSON string) (*AgentCard, error) {
	if cardJSON == "" {
		return nil, fmt.Errorf("AgentCard JSON is empty")
	}
	var raw agentCardJSON
	if err := json.Unmarshal([]byte(cardJSON), &raw); err != nil {
		return nil, err
	}
	card := &AgentCard{
		Name:        raw.Name,
		Description: raw.Description,
		Version:     raw.Version,
		URL:         raw.URL,
	}
	card.SupportedInterfaces = make([]AgentInterface, 0, len(raw.SupportedInterfaces))
	for _, item := range raw.SupportedInterfaces {
		transport := item.ProtocolBinding
		if transport == "" {
			transport = item.Transport
		}
		card.SupportedInterfaces = append(card.SupportedInterfaces, AgentInterface{
			URL:             item.URL,
			Transport:       transport,
			ProtocolVersion: item.ProtocolVersion,
			Tenant:          item.Tenant,
		})
	}
	if len(card.SupportedInterfaces) == 0 && raw.URL != "" {
		card.SupportedInterfaces = append(card.SupportedInterfaces, AgentInterface{
			URL:       raw.URL,
			Transport: raw.PreferredTransport,
		})
		card.legacyURLOnly = true
	}
	card.Skills = make([]AgentSkill, 0, len(raw.Skills))
	for _, item := range raw.Skills {
		card.Skills = append(card.Skills, AgentSkill{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Tags:        append([]string(nil), item.Tags...),
			Examples:    append([]string(nil), item.Examples...),
			InputModes:  append([]string(nil), item.InputModes...),
			OutputModes: append([]string(nil), item.OutputModes...),
			Version:     item.Version,
		})
	}
	return card, nil
}
