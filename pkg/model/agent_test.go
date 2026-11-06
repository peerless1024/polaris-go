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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAgentCardJSON(t *testing.T) string {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"name":        "customer-agent",
		"description": "customer support agent",
		"version":     "1.0.0",
		"supportedInterfaces": []map[string]interface{}{
			{
				"url":             "https://agent.example.com/a2a/",
				"protocolBinding": "JSONRPC",
				"protocolVersion": "1.0",
			},
		},
		"skills": []map[string]interface{}{
			{"id": "customer-agent.answer", "name": "answer", "version": "1.0.0"},
		},
	})
	require.NoError(t, err)
	return string(data)
}

func validAgentMetadata(t *testing.T) map[string]string {
	t.Helper()
	metadata, err := BuildAgentInstanceMetadata(AgentInstanceMetadata{
		AgentName: "customer-agent",
		CardJSON:  validAgentCardJSON(t),
		Transport: "JSONRPC",
		Path:      "/a2a/",
	})
	require.NoError(t, err)
	return metadata
}

func TestParseSkillRef(t *testing.T) {
	namespace, name, version, err := ParseSkillRef("Production:demo-skill@1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "Production", namespace)
	assert.Equal(t, "demo-skill", name)
	assert.Equal(t, "1.2.3", version)

	ref, err := BuildSkillRef(namespace, name, version)
	require.NoError(t, err)
	assert.Equal(t, "Production:demo-skill@1.2.3", ref)

	for _, value := range []string{
		"Production:demo-skill",
		":demo-skill@1.0.0",
		"Production:@1.0.0",
		"Production:demo-skill@",
		"Production:demo@skill@1.0.0",
		"Production:demo:skill@1.0.0",
		" Production:demo-skill@1.0.0",
		"Production:../demo@1.0.0",
	} {
		t.Run("invalid_"+strings.ReplaceAll(value, ":", "_"), func(t *testing.T) {
			_, _, _, parseErr := ParseSkillRef(value)
			require.Error(t, parseErr)
			assert.Equal(t, ErrCodeAPIInvalidArgument, GetErrorCodeFromError(parseErr))
		})
	}
}

func TestBuildSkillRefRejectsReservedDelimiters(t *testing.T) {
	tests := []struct {
		namespace string
		name      string
		version   string
	}{
		{namespace: "Pro:duction", name: "demo", version: "1.0.0"},
		{namespace: "Production", name: "de@mo", version: "1.0.0"},
		{namespace: "Production", name: "demo", version: "b@1"},
		{namespace: "Production", name: "demo", version: "b:1"},
	}
	for _, tt := range tests {
		t.Run(tt.namespace+"/"+tt.name+"/"+tt.version, func(t *testing.T) {
			_, err := BuildSkillRef(tt.namespace, tt.name, tt.version)
			require.Error(t, err)
			assert.Equal(t, ErrCodeAPIInvalidArgument, GetErrorCodeFromError(err))
			assert.Contains(t, err.Error(), "must not contain ':' or '@'")
		})
	}
}

func TestNormalizeAgentSkills(t *testing.T) {
	skills := []*AgentSkill{
		{ID: "Production:z@1.0.0", Name: "z", Version: "1.0.0"},
		{ID: "Production:a@1.0.0"},
	}
	normalized, err := NormalizeAgentSkills(skills)
	require.NoError(t, err)
	assert.Equal(t, "a", normalized[1].Name)
	assert.Equal(t, "1.0.0", normalized[1].Version)

	_, err = NormalizeAgentSkills([]*AgentSkill{
		{ID: "Production:a@1.0.0"},
		{ID: "Production:a@1.0.0"},
	})
	require.Error(t, err)
	assert.Equal(t, ErrCodeAPIInvalidArgument, GetErrorCodeFromError(err))
}

func TestValidateAgentInstanceMetadata(t *testing.T) {
	metadata := validAgentMetadata(t)
	require.NoError(t, ValidateAgentInstanceMetadata(metadata))
	card, err := ParseAgentCard(metadata[MetadataKeyA2AAgentCard])
	require.NoError(t, err)
	assert.Equal(t, "customer-agent", card.Name)
	assert.Equal(t, "1.0.0", card.Version)
	assert.Equal(t, "JSONRPC", card.SupportedInterfaces[0].Transport)

	t.Run("invalid_json", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyA2AAgentCard] = "{"
		err := ValidateAgentInstanceMetadata(invalid)
		require.Error(t, err)
		assert.Equal(t, ErrCodeAPIInvalidArgument, GetErrorCodeFromError(err))
	})
	t.Run("name_mismatch", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyAIAgentName] = "other-agent"
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("path_mismatch", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyA2APath] = "/other/"
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("transport_missing", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		delete(invalid, MetadataKeyA2ATransport)
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("transport_mismatch", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyA2ATransport] = "GRPC"
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("card_transport_missing", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyA2AAgentCard] = `{
			"name":"customer-agent",
			"version":"1.0.0",
			"supportedInterfaces":[{"url":"https://agent.example.com/a2a/"}]
		}`
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("sensitive_key", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid["api-token"] = "secret"
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("too_many_entries", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		for i := len(invalid); i <= MaxInstanceMetadataEntries; i++ {
			invalid["extra-"+string(rune('a'+i))] = "v"
		}
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("card_too_large", func(t *testing.T) {
		invalid := cloneMetadata(metadata)
		invalid[MetadataKeyA2AAgentCard] = strings.Repeat("界", MaxMetadataValueRunes+1)
		require.Error(t, ValidateAgentInstanceMetadata(invalid))
	})
	t.Run("top_level_url", func(t *testing.T) {
		cardJSON := `{"name":"legacy-agent","version":"1.0.0","url":"http://127.0.0.1:8080/a2a/"}`
		legacy := map[string]string{
			MetadataKeyAIAgentName:  "legacy-agent",
			MetadataKeyA2AAgentCard: cardJSON,
			MetadataKeyA2ATransport: "JSONRPC",
			MetadataKeyA2APath:      "/a2a/",
		}
		require.NoError(t, ValidateAgentInstanceMetadata(legacy))
	})
}

func TestBuildAgentInstanceMetadataPreservesCardJSON(t *testing.T) {
	cardJSON := validAgentCardJSON(t)
	metadata, err := BuildAgentInstanceMetadata(AgentInstanceMetadata{
		AgentName: "customer-agent",
		CardJSON:  cardJSON,
		Transport: "JSON-RPC",
		Path:      "/a2a/",
	})
	require.NoError(t, err)
	assert.Equal(t, cardJSON, metadata[MetadataKeyA2AAgentCard])
}

func cloneMetadata(metadata map[string]string) map[string]string {
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[key] = value
	}
	return result
}
