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
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	// MetadataKeyAIAgentName 是实例 metadata 中标识 AI Agent 逻辑名的 key。
	MetadataKeyAIAgentName = "ai-agent-name"
	// MetadataKeyA2AAgentCard 是实例 metadata 中存放 AgentCard JSON 的 key。
	MetadataKeyA2AAgentCard = "a2a.agent.card"
	// MetadataKeyA2ATransport 是实例 metadata 中声明 A2A transport 的 key。
	MetadataKeyA2ATransport = "a2a.transport"
	// MetadataKeyA2APath 是实例 metadata 中声明 A2A endpoint path 的 key。
	MetadataKeyA2APath = "a2a.path"
)

const (
	// MaxInstanceMetadataEntries 是单个实例允许的 metadata 项数上限。
	MaxInstanceMetadataEntries = 64
	// MaxMetadataKeyRunes 是单个 metadata key 的 Unicode 字符上限。
	MaxMetadataKeyRunes = 128
	// MaxMetadataValueRunes 是单个 metadata value 的 Unicode 字符上限。
	MaxMetadataValueRunes = 65535
)

// ServiceType 是协议无关的服务类型。
type ServiceType int32

const (
	// ServiceTypeMicroservice 表示普通微服务。
	ServiceTypeMicroservice ServiceType = 0
	// ServiceTypeMCPServer 表示 MCP Server。
	ServiceTypeMCPServer ServiceType = 1
	// ServiceTypeAIAgent 表示 AI Agent。
	ServiceTypeAIAgent ServiceType = 2
)

// String 返回服务类型的稳定字符串表示。
func (s ServiceType) String() string {
	switch s {
	case ServiceTypeMicroservice:
		return "SERVICE_TYPE_MICROSERVICE"
	case ServiceTypeMCPServer:
		return "SERVICE_TYPE_MCP_SERVER"
	case ServiceTypeAIAgent:
		return "SERVICE_TYPE_AI_AGENT"
	default:
		return fmt.Sprintf("SERVICE_TYPE_UNKNOWN(%d)", int32(s))
	}
}

// ExtendedMetadataType 是服务扩展元数据类型。
type ExtendedMetadataType int32

const (
	// ExtendedMetadataUnknown 表示当前 SDK 不理解的扩展类型。
	ExtendedMetadataUnknown ExtendedMetadataType = 0
	// ExtendedMetadataSkill 表示 Agent Skill 引用。
	ExtendedMetadataSkill ExtendedMetadataType = 1
)

// AgentSkill 是 Agent Skill 的结构化描述。
type AgentSkill struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	Examples    []string
	InputModes  []string
	OutputModes []string
	Version     string
}

// ExtendedMetadata 是服务级扩展元数据。
type ExtendedMetadata struct {
	Type       ExtendedMetadataType
	AgentSkill *AgentSkill
}

// AgentService 是服务发现返回的完整 AI Agent 服务快照。
type AgentService struct {
	Namespace        string
	Name             string
	Type             ServiceType
	Metadata         map[string]string
	ExtendedMetadata []*ExtendedMetadata
	Owners           string
	Revision         string
}

// GetService 返回服务名。
func (a *AgentService) GetService() string {
	if a == nil {
		return ""
	}
	return a.Name
}

// GetNamespace 返回命名空间。
func (a *AgentService) GetNamespace() string {
	if a == nil {
		return ""
	}
	return a.Namespace
}

// GetMetadata 返回服务 metadata。
func (a *AgentService) GetMetadata() map[string]string {
	if a == nil {
		return nil
	}
	return a.Metadata
}

// AgentInstanceMetadata 描述 AI Agent 实例注册所需的 metadata 四元组。
type AgentInstanceMetadata struct {
	AgentName string
	CardJSON  string
	Transport string
	Path      string
}

// EnsureAgentServiceRequest 确保 AI Agent 服务存在；服务已存在时不更新 metadata/Skill。
type EnsureAgentServiceRequest struct {
	Namespace string
	Service   string
	Metadata  map[string]string
	Skills    []AgentSkill
}

// Validate 校验 EnsureAgentServiceRequest。
func (r *EnsureAgentServiceRequest) Validate() error {
	if r == nil {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil, "EnsureAgentServiceRequest can not be nil")
	}
	if r.Namespace == "" || r.Service == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"EnsureAgentServiceRequest namespace and service must not be empty")
	}
	_, err := NormalizeAgentSkillValues(r.Skills)
	return err
}

// MountAgentSkillsRequest 挂载 AI Agent Skill。
type MountAgentSkillsRequest struct {
	Namespace string
	Service   string
	Skills    []AgentSkill
	Replace   bool
}

// Validate 校验 MountAgentSkillsRequest。
func (r *MountAgentSkillsRequest) Validate() error {
	if r == nil {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil, "MountAgentSkillsRequest can not be nil")
	}
	if r.Namespace == "" || r.Service == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"MountAgentSkillsRequest namespace and service must not be empty")
	}
	_, err := NormalizeAgentSkillValues(r.Skills)
	return err
}

// UnmountAgentSkillsRequest 卸载 AI Agent Skill。
type UnmountAgentSkillsRequest struct {
	Namespace string
	Service   string
	SkillIDs  []string
}

// Validate 校验 UnmountAgentSkillsRequest。
func (r *UnmountAgentSkillsRequest) Validate() error {
	if r == nil {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil, "UnmountAgentSkillsRequest can not be nil")
	}
	if r.Namespace == "" || r.Service == "" || len(r.SkillIDs) == 0 {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"UnmountAgentSkillsRequest namespace, service and skill ids must not be empty")
	}
	for _, id := range r.SkillIDs {
		if _, _, _, err := ParseSkillRef(id); err != nil {
			return err
		}
	}
	return nil
}

// BuildSkillRef 构建 namespace:name@version 形式的 Skill 引用。
func BuildSkillRef(namespace, name, version string) (string, error) {
	if err := ValidateSkillRefParts(namespace, name, version); err != nil {
		return "", err
	}
	return namespace + ":" + name + "@" + version, nil
}

// ValidateSkillRefParts 校验 namespace、name 和 version 是安全的引用片段。
func ValidateSkillRefParts(namespace, name, version string) error {
	if err := validateSkillSegment("namespace", namespace); err != nil {
		return err
	}
	if err := validateSkillSegment("name", name); err != nil {
		return err
	}
	return validateSkillSegment("version", version)
}

// ParseSkillRef 解析 namespace:name@version 形式的 Skill 引用。
func ParseSkillRef(value string) (namespace, name, version string, err error) {
	if strings.TrimSpace(value) != value || value == "" {
		return "", "", "", NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"skill ref must be non-empty and must not contain surrounding whitespace")
	}
	if strings.Count(value, ":") != 1 || strings.Count(value, "@") != 1 {
		return "", "", "", NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"skill ref %q must use namespace:name@version", value)
	}
	parts := strings.SplitN(value, ":", 2)
	nameAndVersion := strings.SplitN(parts[1], "@", 2)
	namespace, name, version = parts[0], nameAndVersion[0], nameAndVersion[1]
	if err = ValidateSkillRefParts(namespace, name, version); err != nil {
		return "", "", "", err
	}
	return namespace, name, version, nil
}

// NormalizeAgentSkills 校验 Skill 引用、补齐 name/version 并按 ID 去重。
func NormalizeAgentSkills(skills []*AgentSkill) ([]*AgentSkill, error) {
	result := make([]*AgentSkill, 0, len(skills))
	seen := make(map[string]struct{}, len(skills))
	for _, skill := range skills {
		if skill == nil {
			return nil, NewSDKError(ErrCodeAPIInvalidArgument, nil, "agent skill must not be nil")
		}
		_, parsedName, parsedVersion, err := ParseSkillRef(skill.ID)
		if err != nil {
			return nil, NewSDKError(ErrCodeAPIInvalidArgument, err, "invalid agent skill id %q", skill.ID)
		}
		if skill.Name != "" && skill.Name != parsedName {
			return nil, NewSDKError(ErrCodeAPIInvalidArgument, nil,
				"agent skill %q name %q does not match id", skill.ID, skill.Name)
		}
		if skill.Version != "" && skill.Version != parsedVersion {
			return nil, NewSDKError(ErrCodeAPIInvalidArgument, nil,
				"agent skill %q version %q does not match id", skill.ID, skill.Version)
		}
		if _, ok := seen[skill.ID]; ok {
			return nil, NewSDKError(ErrCodeAPIInvalidArgument, nil, "duplicate agent skill %q", skill.ID)
		}
		seen[skill.ID] = struct{}{}
		copySkill := *skill
		copySkill.Name = parsedName
		copySkill.Version = parsedVersion
		copySkill.Tags = append([]string(nil), skill.Tags...)
		copySkill.Examples = append([]string(nil), skill.Examples...)
		copySkill.InputModes = append([]string(nil), skill.InputModes...)
		copySkill.OutputModes = append([]string(nil), skill.OutputModes...)
		result = append(result, &copySkill)
	}
	return result, nil
}

// NormalizeAgentSkillValues 校验值类型 Skill 列表并按 ID 去重。
func NormalizeAgentSkillValues(skills []AgentSkill) ([]*AgentSkill, error) {
	pointers := make([]*AgentSkill, 0, len(skills))
	for index := range skills {
		skill := skills[index]
		pointers = append(pointers, &skill)
	}
	return NormalizeAgentSkills(pointers)
}

// BuildAgentInstanceMetadata 构建并校验 AI Agent 实例 metadata。
func BuildAgentInstanceMetadata(metadata AgentInstanceMetadata) (map[string]string, error) {
	result := map[string]string{
		MetadataKeyAIAgentName:  metadata.AgentName,
		MetadataKeyA2AAgentCard: metadata.CardJSON,
		MetadataKeyA2ATransport: metadata.Transport,
		MetadataKeyA2APath:      metadata.Path,
	}
	if err := ValidateAgentInstanceMetadata(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateAgentInstanceMetadata 校验 AI Agent 实例 metadata 的最小协议契约。
func ValidateAgentInstanceMetadata(metadata map[string]string) error {
	if len(metadata) > MaxInstanceMetadataEntries {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata has %d entries, maximum is %d", len(metadata), MaxInstanceMetadataEntries)
	}
	for key, value := range metadata {
		if key == "" {
			return NewSDKError(ErrCodeAPIInvalidArgument, nil, "instance metadata key must not be empty")
		}
		keyRunes := utf8.RuneCountInString(key)
		if keyRunes > MaxMetadataKeyRunes {
			return NewSDKError(ErrCodeAPIInvalidArgument, nil,
				"instance metadata key %q has %d characters, maximum is %d",
				key, keyRunes, MaxMetadataKeyRunes)
		}
		valueRunes := utf8.RuneCountInString(value)
		if valueRunes > MaxMetadataValueRunes {
			return NewSDKError(ErrCodeAPIInvalidArgument, nil,
				"instance metadata value for %q has %d characters, maximum is %d",
				key, valueRunes, MaxMetadataValueRunes)
		}
		lowerKey := strings.ToLower(key)
		for _, sensitive := range []string{"token", "secret", "password", "authorization"} {
			if strings.Contains(lowerKey, sensitive) {
				return NewSDKError(ErrCodeAPIInvalidArgument, nil,
					"instance metadata key %q looks sensitive", key)
			}
		}
	}

	agentName := strings.TrimSpace(metadata[MetadataKeyAIAgentName])
	if agentName == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q must not be empty", MetadataKeyAIAgentName)
	}
	cardJSON := metadata[MetadataKeyA2AAgentCard]
	if cardJSON == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q must not be empty", MetadataKeyA2AAgentCard)
	}
	card, err := ParseAgentCard(cardJSON)
	if err != nil {
		return NewSDKError(ErrCodeAPIInvalidArgument, err, "invalid AgentCard")
	}
	if card.Name != agentName {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"AgentCard name %q does not match %q value %q",
			card.Name, MetadataKeyAIAgentName, agentName)
	}
	if strings.TrimSpace(card.Version) == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil, "AgentCard version must not be empty")
	}
	primary, ok := card.PrimaryInterface()
	if !ok || strings.TrimSpace(primary.URL) == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"AgentCard must contain at least one supported interface or a top-level url")
	}
	endpoint, err := url.Parse(primary.URL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, err,
			"AgentCard endpoint %q must be an absolute URL", primary.URL)
	}

	transport := strings.TrimSpace(metadata[MetadataKeyA2ATransport])
	if transport == "" {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q must not be empty", MetadataKeyA2ATransport)
	}
	if primary.Transport != "" && normalizeTransport(primary.Transport) != normalizeTransport(transport) {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q value %q does not match AgentCard transport %q",
			MetadataKeyA2ATransport, transport, primary.Transport)
	}
	if primary.Transport == "" && !card.legacyURLOnly {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"AgentCard primary supported interface must declare a protocolBinding")
	}

	path := metadata[MetadataKeyA2APath]
	if !strings.HasPrefix(path, "/") {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q value %q must start with '/'", MetadataKeyA2APath, path)
	}
	if endpoint.Path != path {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"instance metadata %q value %q does not match AgentCard path %q",
			MetadataKeyA2APath, path, endpoint.Path)
	}
	return nil
}

func validateSkillSegment(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"skill %s must be non-empty and must not contain surrounding whitespace", field)
	}
	if value == "." || value == ".." || strings.ContainsAny(value, "/\\:@") {
		return NewSDKError(ErrCodeAPIInvalidArgument, nil,
			"skill %s %q is not a safe path segment and must not contain ':' or '@'", field, value)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return NewSDKError(ErrCodeAPIInvalidArgument, nil,
				"skill %s contains control characters", field)
		}
	}
	return nil
}

func normalizeTransport(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}
