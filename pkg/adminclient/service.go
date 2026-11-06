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

package adminclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/polarismesh/polaris-go/pkg/model"
	modelpb "github.com/polarismesh/polaris-go/pkg/model/pb"
)

type serviceRecord struct {
	Service *apiservice.Service
	Raw     map[string]json.RawMessage
}

type serviceWriteState struct {
	confirmed bool
	extended  []json.RawMessage
	pending   *serviceWritePayload
	method    string
	uncertain bool
}

const serviceReadAfterWriteInterval = 100 * time.Millisecond

type serviceWritePayload struct {
	Name                 string             `json:"name"`
	Namespace            string             `json:"namespace"`
	ServiceType          string             `json:"service_type,omitempty"`
	Metadata             map[string]string  `json:"metadata,omitempty"`
	ExtendedMetadata     *[]json.RawMessage `json:"extended_metadata,omitempty"`
	Revision             string             `json:"revision,omitempty"`
	Owners               string             `json:"owners,omitempty"`
	ExportTo             []string           `json:"export_to,omitempty"`
	SyncToGlobalRegistry bool               `json:"sync_to_global_registry"`
}

type extendedMetadataPayload struct {
	Type            json.RawMessage `json:"type"`
	AgentSkill      json.RawMessage `json:"agent_skill,omitempty"`
	AgentSkillCamel json.RawMessage `json:"agentSkill,omitempty"`
}

type agentSkillPayload struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"input_modes,omitempty"`
	OutputModes []string `json:"output_modes,omitempty"`
	Version     string   `json:"version,omitempty"`
}

type queryEnvelope struct {
	Code     uint32            `json:"code"`
	Info     string            `json:"info"`
	Services []json.RawMessage `json:"services"`
}

type writeEnvelope struct {
	Code      uint32            `json:"code"`
	Info      string            `json:"info"`
	Responses []json.RawMessage `json:"responses"`
}

type responseEnvelope struct {
	Code    uint32          `json:"code"`
	Info    string          `json:"info"`
	Service json.RawMessage `json:"service"`
}

// GetService 从 Console 精确读取最新服务。
func (c *Client) GetService(namespace, service string) (*serviceRecord, error) {
	ctx, cancel := c.context()
	defer cancel()
	return c.getService(ctx, namespace, service)
}

func (c *Client) getService(ctx context.Context, namespace, service string) (*serviceRecord, error) {
	query := url.Values{}
	query.Set("namespace", namespace)
	query.Set("name", service)
	body, status, err := c.doJSON(ctx, http.MethodGet, serviceRequestPath, query, nil)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, model.NewSDKError(model.ErrCodeServiceNotFound, err,
				"service %s/%s not found", namespace, service)
		}
		return nil, err
	}
	var envelope queryEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode Console service query response")
	}
	if envelope.Code == uint32(apimodel.Code_NotFoundResource) {
		return nil, model.NewSDKError(model.ErrCodeServiceNotFound, nil,
			"service %s/%s not found", namespace, service)
	}
	if envelope.Code != uint32(apimodel.Code_ExecuteSuccess) {
		return nil, model.NewSDKErrorWithServerInfo(model.ErrCodeServiceWriteError, nil,
			envelope.Code, envelope.Info, "Console service query returned an error")
	}
	for _, rawService := range envelope.Services {
		record, recordErr := decodeService(rawService)
		if recordErr != nil {
			return nil, recordErr
		}
		if record.Service.GetNamespace().GetValue() == namespace &&
			record.Service.GetName().GetValue() == service {
			return record, nil
		}
	}
	return nil, model.NewSDKError(model.ErrCodeServiceNotFound, nil,
		"service %s/%s not found", namespace, service)
}

// EnsureAgentService 确保 AI Agent 服务存在。
func (c *Client) EnsureAgentService(req *model.EnsureAgentServiceRequest) (*serviceRecord, error) {
	ctx, cancel := c.context()
	defer cancel()
	unlock, err := c.lockServiceWrite(ctx, req.Namespace, req.Service)
	if err != nil {
		return nil, err
	}
	defer unlock()
	record, err := c.getService(ctx, req.Namespace, req.Service)
	if err == nil {
		return record, validateAgentServiceType(record, req.Namespace, req.Service)
	}
	if model.GetErrorCodeFromError(err) != model.ErrCodeServiceNotFound {
		return nil, err
	}
	skills, normalizeErr := model.NormalizeAgentSkillValues(req.Skills)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	extended, err := buildSkillExtendedMetadata(skills)
	if err != nil {
		return nil, err
	}
	payload := serviceWritePayload{
		Name:             req.Service,
		Namespace:        req.Namespace,
		ServiceType:      apiservice.ServiceType_SERVICE_TYPE_AI_AGENT.String(),
		Metadata:         cloneMap(req.Metadata),
		ExtendedMetadata: &extended,
	}
	writeErr := c.writeService(ctx, http.MethodPost, payload, false)
	if writeErr != nil {
		if model.GetErrorCodeFromError(writeErr) == model.ErrCodeServiceWriteError {
			existing, getErr := c.getService(ctx, req.Namespace, req.Service)
			if getErr == nil {
				if existing.Service.GetServiceType() != apiservice.ServiceType_SERVICE_TYPE_AI_AGENT {
					return nil, model.NewSDKError(model.ErrCodeServiceTypeConflict, writeErr,
						"service %s/%s already exists with type %s",
						req.Namespace, req.Service, existing.Service.GetServiceType())
				}
				return existing, nil
			}
		}
		return nil, writeErr
	}
	record, err = c.getServiceAfterWrite(ctx, req.Namespace, req.Service, extended)
	if err != nil {
		return nil, err
	}
	c.rememberConfirmedExtendedMetadata(req.Namespace, req.Service, extended)
	return record, validateAgentServiceType(record, req.Namespace, req.Service)
}

// MountAgentSkills 合并或整体替换服务级 Skill。
func (c *Client) MountAgentSkills(
	namespace, service string, skills []*model.AgentSkill, replace bool,
) (*serviceRecord, error) {
	ctx, cancel := c.context()
	defer cancel()
	unlock, err := c.lockServiceWrite(ctx, namespace, service)
	if err != nil {
		return nil, err
	}
	defer unlock()
	record, err := c.getServiceForUpdate(ctx, namespace, service)
	if err != nil {
		return nil, err
	}
	if record.Service.GetServiceType() != apiservice.ServiceType_SERVICE_TYPE_AI_AGENT {
		return nil, model.NewSDKError(model.ErrCodeServiceTypeConflict, nil,
			"service %s/%s is not AI_AGENT", namespace, service)
	}
	normalized, err := model.NormalizeAgentSkills(skills)
	if err != nil {
		return nil, err
	}
	extended, err := mergeExtendedMetadata(record.Raw, normalized, replace, nil)
	if err != nil {
		return nil, err
	}
	return c.updateExtendedMetadata(ctx, record, extended)
}

// UnmountAgentSkills 删除指定 ID 的 Skill 引用。
func (c *Client) UnmountAgentSkills(
	namespace, service string, skillIDs []string,
) (*serviceRecord, error) {
	ctx, cancel := c.context()
	defer cancel()
	unlock, err := c.lockServiceWrite(ctx, namespace, service)
	if err != nil {
		return nil, err
	}
	defer unlock()
	record, err := c.getServiceForUpdate(ctx, namespace, service)
	if err != nil {
		return nil, err
	}
	if record.Service.GetServiceType() != apiservice.ServiceType_SERVICE_TYPE_AI_AGENT {
		return nil, model.NewSDKError(model.ErrCodeServiceTypeConflict, nil,
			"service %s/%s is not AI_AGENT", namespace, service)
	}
	extended, err := mergeExtendedMetadata(record.Raw, nil, false, skillIDs)
	if err != nil {
		return nil, err
	}
	return c.updateExtendedMetadata(ctx, record, extended)
}

func (c *Client) updateExtendedMetadata(
	ctx context.Context, record *serviceRecord, extended []json.RawMessage,
) (*serviceRecord, error) {
	if len(extended) == 0 {
		matched, err := extendedMetadataMatches(record, extended)
		if err != nil {
			return nil, err
		}
		if matched {
			namespace := record.Service.GetNamespace().GetValue()
			service := record.Service.GetName().GetValue()
			c.rememberConfirmedExtendedMetadata(namespace, service, extended)
			return record, nil
		}
	}
	payload := serviceWritePayload{
		Name:                 record.Service.GetName().GetValue(),
		Namespace:            record.Service.GetNamespace().GetValue(),
		ServiceType:          record.Service.GetServiceType().String(),
		ExtendedMetadata:     &extended,
		Revision:             record.Service.GetRevision().GetValue(),
		ExportTo:             serviceExportTo(record.Service.GetExportTo()),
		SyncToGlobalRegistry: record.Service.GetSyncToGlobalRegistry(),
	}
	if err := c.writeService(ctx, http.MethodPut, payload, len(extended) > 0); err != nil {
		return nil, err
	}
	record, err := c.getServiceAfterWrite(ctx, payload.Namespace, payload.Name, extended)
	if err != nil {
		return nil, err
	}
	c.rememberConfirmedExtendedMetadata(payload.Namespace, payload.Name, extended)
	return record, nil
}

func (c *Client) getServiceForUpdate(
	ctx context.Context, namespace, service string,
) (*serviceRecord, error) {
	state, ok := c.serviceWriteStateForUpdate(namespace, service)
	if !ok {
		return c.getService(ctx, namespace, service)
	}
	expected := state.extended
	if state.pending != nil {
		expected = *state.pending.ExtendedMetadata
	}
	var lastErr error
	for {
		record, err := c.getService(ctx, namespace, service)
		if err == nil {
			matched, matchErr := extendedMetadataMatches(record, expected)
			if matchErr != nil {
				return nil, matchErr
			}
			if matched {
				c.rememberConfirmedExtendedMetadata(namespace, service, expected)
				return record, nil
			}
			lastErr = model.NewSDKError(model.ErrCodeServiceWriteError, nil,
				"service %s/%s has not reached the last confirmed extended metadata", namespace, service)
		} else if model.GetErrorCodeFromError(err) != model.ErrCodeServiceNotFound {
			return nil, err
		} else {
			lastErr = err
		}
		if state.pending != nil && state.uncertain {
			if err = c.writeService(ctx, state.method, *state.pending, len(expected) > 0); err != nil {
				return nil, err
			}
			state.uncertain = false
			continue
		}
		select {
		case <-ctx.Done():
			confirmErr := ctx.Err()
			if lastErr != nil {
				confirmErr = lastErr
			}
			return nil, model.NewSDKError(model.ErrCodeServiceWriteError, confirmErr,
				"previous service write for %s/%s was not confirmed before timeout", namespace, service)
		case <-time.After(serviceReadAfterWriteInterval):
		}
	}
}

func (c *Client) serviceWriteStateForUpdate(namespace, service string) (serviceWriteState, bool) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	state, ok := c.serviceStates[serviceStateKey(namespace, service)]
	if !ok || (!state.confirmed && state.pending == nil) {
		return serviceWriteState{}, false
	}
	state.extended = cloneRawMessages(state.extended)
	if state.pending != nil {
		payload := *state.pending
		extended := cloneRawMessages(*payload.ExtendedMetadata)
		payload.ExtendedMetadata = &extended
		state.pending = &payload
	}
	return state, true
}

func (c *Client) rememberPendingServiceWrite(method string, payload serviceWritePayload, uncertain bool) {
	if payload.ExtendedMetadata == nil {
		return
	}
	values := cloneRawMessages(*payload.ExtendedMetadata)
	payload.ExtendedMetadata = &values
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	if c.serviceStates == nil {
		c.serviceStates = make(map[string]serviceWriteState)
	}
	key := serviceStateKey(payload.Namespace, payload.Name)
	state := c.serviceStates[key]
	state.pending = &payload
	state.method = method
	state.uncertain = uncertain
	c.serviceStates[key] = state
}

func (c *Client) rememberConfirmedExtendedMetadata(
	namespace, service string, extended []json.RawMessage,
) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	if c.serviceStates == nil {
		c.serviceStates = make(map[string]serviceWriteState)
	}
	c.serviceStates[serviceStateKey(namespace, service)] = serviceWriteState{
		confirmed: true,
		extended:  cloneRawMessages(extended),
	}
}

func serviceStateKey(namespace, service string) string {
	return namespace + "\x00" + service
}

func cloneRawMessages(values []json.RawMessage) []json.RawMessage {
	if values == nil {
		return nil
	}
	result := make([]json.RawMessage, len(values))
	for index := range values {
		result[index] = append(json.RawMessage(nil), values[index]...)
	}
	return result
}

func (c *Client) getServiceAfterWrite(
	ctx context.Context, namespace, service string, expectedExtended []json.RawMessage,
) (*serviceRecord, error) {
	var lastErr error
	for {
		record, err := c.getService(ctx, namespace, service)
		if err == nil {
			matched, matchErr := extendedMetadataMatches(record, expectedExtended)
			if matchErr != nil {
				return nil, matchErr
			}
			if matched {
				return record, nil
			}
			lastErr = nil
		} else if model.GetErrorCodeFromError(err) != model.ErrCodeServiceNotFound {
			return nil, err
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			confirmErr := ctx.Err()
			if lastErr != nil {
				confirmErr = lastErr
			}
			return nil, model.NewSDKError(model.ErrCodeServiceWriteError, confirmErr,
				"service write for %s/%s was not confirmed before timeout", namespace, service)
		case <-time.After(serviceReadAfterWriteInterval):
		}
	}
}

func serviceExportTo(values []*wrapperspb.StringValue) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.GetValue())
	}
	return result
}

func (c *Client) writeService(ctx context.Context, method string, payload serviceWritePayload, idempotentNoNeed bool) error {
	result := c.doJSONWithAttempts(ctx, method, serviceRequestPath, nil, []serviceWritePayload{payload})
	if result.err != nil {
		if result.uncertain || result.status >= http.StatusInternalServerError {
			c.rememberPendingServiceWrite(method, payload, true)
		}
		return result.err
	}
	var envelope writeEnvelope
	if err := json.Unmarshal(result.body, &envelope); err != nil {
		c.rememberPendingServiceWrite(method, payload, true)
		return model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode Console service write response")
	}
	if envelope.Code == uint32(apimodel.Code_NoNeedUpdate) {
		if idempotentNoNeed {
			c.rememberPendingServiceWrite(method, payload, result.uncertain)
			return nil
		}
		if result.uncertain {
			c.rememberPendingServiceWrite(method, payload, true)
		}
		return model.NewSDKErrorWithServerInfo(model.ErrCodeServiceWriteUnsupported, nil,
			envelope.Code, envelope.Info,
			"Console service write was not applied; explicit empty extended_metadata requires server support")
	}
	if envelope.Code != uint32(apimodel.Code_ExecuteSuccess) {
		if result.uncertain {
			c.rememberPendingServiceWrite(method, payload, true)
		}
		return model.NewSDKErrorWithServerInfo(model.ErrCodeServiceWriteError, nil,
			envelope.Code, envelope.Info, "Console service write returned an error")
	}
	if len(envelope.Responses) == 0 {
		c.rememberPendingServiceWrite(method, payload, true)
		return model.NewSDKError(model.ErrCodeServiceWriteError, nil,
			"Console service write returned no item response")
	}
	var response responseEnvelope
	if err := json.Unmarshal(envelope.Responses[0], &response); err != nil {
		c.rememberPendingServiceWrite(method, payload, true)
		return model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode Console service write item")
	}
	if response.Code == uint32(apimodel.Code_NoNeedUpdate) {
		if idempotentNoNeed {
			c.rememberPendingServiceWrite(method, payload, result.uncertain)
			return nil
		}
		if result.uncertain {
			c.rememberPendingServiceWrite(method, payload, true)
		}
		return model.NewSDKErrorWithServerInfo(model.ErrCodeServiceWriteUnsupported, nil,
			response.Code, response.Info,
			"Console service write was not applied; explicit empty extended_metadata requires server support")
	}
	if response.Code != uint32(apimodel.Code_ExecuteSuccess) {
		if result.uncertain {
			c.rememberPendingServiceWrite(method, payload, true)
		}
		return model.NewSDKErrorWithServerInfo(model.ErrCodeServiceWriteError, nil,
			response.Code, response.Info, "Console service write item returned an error")
	}
	c.rememberPendingServiceWrite(method, payload, false)
	return nil
}

func decodeService(rawService json.RawMessage) (*serviceRecord, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawService, &raw); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode raw Console service")
	}
	serviceJSON, err := serviceWithoutExtendedMetadata(raw)
	if err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to prepare Console service for decoding")
	}
	service := &apiservice.Service{}
	unmarshaler := &jsonpb.Unmarshaler{AllowUnknownFields: true}
	if err = unmarshaler.Unmarshal(strings.NewReader(string(serviceJSON)), service); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode Console service")
	}
	service.ExtendedMetadata, err = decodeKnownExtendedMetadata(raw)
	if err != nil {
		return nil, err
	}
	return &serviceRecord{Service: service, Raw: raw}, nil
}

func serviceWithoutExtendedMetadata(raw map[string]json.RawMessage) (json.RawMessage, error) {
	filtered := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		if key == "extended_metadata" || key == "extendedMetadata" {
			continue
		}
		filtered[key] = value
	}
	return json.Marshal(filtered)
}

func decodeKnownExtendedMetadata(raw map[string]json.RawMessage) ([]*apiservice.ExtendedMetadata, error) {
	value, ok := raw["extended_metadata"]
	if !ok {
		value, ok = raw["extendedMetadata"]
	}
	if !ok || len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(value, &entries); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode extended_metadata")
	}
	result := make([]*apiservice.ExtendedMetadata, 0, len(entries))
	for _, entry := range entries {
		envelope, _, isSkill, err := decodeExtendedMetadata(entry)
		if err != nil {
			return nil, err
		}
		if !isSkill {
			continue
		}
		agentSkill, err := decodeAgentSkillPayload(envelope.AgentSkill, envelope.AgentSkillCamel)
		if err != nil {
			return nil, err
		}
		result = append(result, &apiservice.ExtendedMetadata{
			Type:       apiservice.ExtendedMetadata_EXTENDED_METADATA_SKILL,
			AgentSkill: agentSkill,
		})
	}
	return result, nil
}

func decodeAgentSkillPayload(values ...json.RawMessage) (*apiservice.AgentSkill, error) {
	var raw json.RawMessage
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			raw = value
			break
		}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode extended AgentSkill")
	}
	// The specification pins snake_case JSON names; accept A2A mode aliases too.
	for _, alias := range []struct{ original, alternate string }{
		{original: "input_modes", alternate: "inputModes"},
		{original: "output_modes", alternate: "outputModes"},
	} {
		if _, exists := fields[alias.original]; !exists {
			if value, ok := fields[alias.alternate]; ok {
				fields[alias.original] = value
			}
		}
		delete(fields, alias.alternate)
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to normalize extended AgentSkill")
	}
	skill := &apiservice.AgentSkill{}
	unmarshaler := &jsonpb.Unmarshaler{AllowUnknownFields: true}
	if err = unmarshaler.Unmarshal(strings.NewReader(string(normalized)), skill); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode extended AgentSkill")
	}
	return skill, nil
}

func extendedMetadataMatches(record *serviceRecord, expected []json.RawMessage) (bool, error) {
	actual, err := recordExtendedMetadata(record)
	if err != nil {
		return false, err
	}
	if len(expected) == 0 && len(actual) == 0 {
		return true, nil
	}
	if len(expected) != len(actual) {
		return false, nil
	}
	canonicalExpected, err := normalizeExtendedMetadataForCompare(expected)
	if err != nil {
		return false, err
	}
	canonicalActual, err := normalizeExtendedMetadataForCompare(actual)
	if err != nil {
		return false, err
	}
	for index := range canonicalExpected {
		if canonicalExpected[index] != canonicalActual[index] {
			return false, nil
		}
	}
	return true, nil
}

func recordExtendedMetadata(record *serviceRecord) ([]json.RawMessage, error) {
	if record == nil {
		return nil, nil
	}
	value, ok := record.Raw["extended_metadata"]
	if !ok {
		value, ok = record.Raw["extendedMetadata"]
	}
	if !ok || len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(value, &entries); err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode extended_metadata for write confirmation")
	}
	return entries, nil
}

func normalizeExtendedMetadataForCompare(values []json.RawMessage) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		envelope, _, isSkill, err := decodeExtendedMetadata(value)
		if err != nil {
			return nil, err
		}
		var normalized interface{}
		prefix := "raw:"
		if isSkill {
			prefix = "skill:"
			skill, skillErr := decodeAgentSkillPayload(envelope.AgentSkill, envelope.AgentSkillCamel)
			if skillErr != nil {
				return nil, skillErr
			}
			if skill != nil {
				skill, skillErr = normalizeAgentSkillForCompare(skill)
				if skillErr != nil {
					return nil, skillErr
				}
			}
			normalized = skill
		} else {
			decoder := json.NewDecoder(strings.NewReader(string(value)))
			decoder.UseNumber()
			if err = decoder.Decode(&normalized); err != nil {
				return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
					"fail to normalize extended metadata")
			}
		}
		canonical, err := json.Marshal(normalized)
		if err != nil {
			return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
				"fail to encode normalized extended metadata")
		}
		result = append(result, prefix+string(canonical))
	}
	// Storage may sort metadata by type and key instead of retaining write order.
	sort.Strings(result)
	return result, nil
}

func normalizeAgentSkillForCompare(skill *apiservice.AgentSkill) (*apiservice.AgentSkill, error) {
	_, name, version, err := model.ParseSkillRef(skill.Id)
	if err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to normalize extended AgentSkill %q", skill.Id)
	}
	result := proto.Clone(skill).(*apiservice.AgentSkill)
	if result.Name == "" {
		result.Name = name
	}
	if result.Version == "" {
		result.Version = version
	}
	return result, nil
}

func mergeExtendedMetadata(
	raw map[string]json.RawMessage, skills []*model.AgentSkill, replace bool, removeIDs []string,
) ([]json.RawMessage, error) {
	var existing []json.RawMessage
	value, ok := raw["extended_metadata"]
	if !ok {
		value, ok = raw["extendedMetadata"]
	}
	if ok && string(value) != "null" {
		if err := json.Unmarshal(value, &existing); err != nil {
			return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
				"fail to decode extended_metadata")
		}
	}
	result := make([]json.RawMessage, 0, len(existing)+len(skills))
	skillIndex := make(map[string]int)
	removed := make(map[string]struct{}, len(removeIDs))
	for _, id := range removeIDs {
		removed[id] = struct{}{}
	}
	for _, item := range existing {
		envelope, id, isSkill, err := decodeExtendedMetadata(item)
		if err != nil {
			return nil, err
		}
		_ = envelope
		if !isSkill {
			result = append(result, item)
			continue
		}
		if _, ok := removed[id]; ok {
			continue
		}
		if replace {
			continue
		}
		skillIndex[id] = len(result)
		result = append(result, item)
	}
	for _, skill := range skills {
		item, err := encodeSkillExtendedMetadata(skill)
		if err != nil {
			return nil, err
		}
		if index, ok := skillIndex[skill.ID]; ok {
			result[index] = item
			continue
		}
		skillIndex[skill.ID] = len(result)
		result = append(result, item)
	}
	return result, nil
}

func decodeExtendedMetadata(raw json.RawMessage) (*extendedMetadataPayload, string, bool, error) {
	var envelope extendedMetadataPayload
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, "", false, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to decode extended metadata item")
	}
	typeValue := strings.Trim(string(envelope.Type), `"`)
	if typeValue != "1" && !strings.EqualFold(typeValue, "EXTENDED_METADATA_SKILL") {
		return &envelope, "", false, nil
	}
	skill, err := decodeAgentSkillPayload(envelope.AgentSkill, envelope.AgentSkillCamel)
	if err != nil {
		return nil, "", false, err
	}
	return &envelope, skill.GetId(), true, nil
}

func buildSkillExtendedMetadata(skills []*model.AgentSkill) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(skills))
	for _, skill := range skills {
		item, err := encodeSkillExtendedMetadata(skill)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func encodeSkillExtendedMetadata(skill *model.AgentSkill) (json.RawMessage, error) {
	skillPayload, err := json.Marshal(agentSkillPayload{
		ID:          skill.ID,
		Name:        skill.Name,
		Description: skill.Description,
		Tags:        skill.Tags,
		Examples:    skill.Examples,
		InputModes:  skill.InputModes,
		OutputModes: skill.OutputModes,
		Version:     skill.Version,
	})
	if err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to encode AgentSkill")
	}
	raw, err := json.Marshal(struct {
		Type       int             `json:"type"`
		AgentSkill json.RawMessage `json:"agent_skill"`
	}{
		Type:       int(model.ExtendedMetadataSkill),
		AgentSkill: skillPayload,
	})
	if err != nil {
		return nil, model.NewSDKError(model.ErrCodeServiceWriteError, err,
			"fail to encode extended AgentSkill")
	}
	return raw, nil
}

func cloneMap(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func validateAgentServiceType(record *serviceRecord, namespace, service string) error {
	if record == nil || record.Service == nil {
		return model.NewSDKError(model.ErrCodeServiceWriteError, nil,
			"Console returned an empty service for %s/%s", namespace, service)
	}
	if record.Service.GetServiceType() != apiservice.ServiceType_SERVICE_TYPE_AI_AGENT {
		return model.NewSDKError(model.ErrCodeServiceTypeConflict, nil,
			"service %s/%s already exists with type %s",
			namespace, service, record.Service.GetServiceType())
	}
	return nil
}

func (r *serviceRecord) AgentService() *model.AgentService {
	if r == nil || r.Service == nil {
		return nil
	}
	return modelpb.NewAgentServiceFromProto(r.Service)
}
