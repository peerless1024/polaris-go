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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillAndServiceWriterDefaults(t *testing.T) {
	cfg := NewDefaultConfiguration([]string{"127.0.0.1:8091"})
	assert.False(t, cfg.GetSkill().IsEnable())
	assert.Empty(t, cfg.GetSkill().GetAddresses())
	assert.Equal(t, DefaultSkillConnectTimeout, cfg.GetSkill().GetConnectTimeout())
	assert.Equal(t, DefaultSkillMessageTimeout, cfg.GetSkill().GetMessageTimeout())
	assert.Equal(t, DefaultSkillDownloadTimeout, cfg.GetSkill().GetDownloadTimeout())
	assert.Empty(t, cfg.GetServiceWriter().GetAddresses())
	assert.Equal(t, DefaultServiceWriterTimeout, cfg.GetServiceWriter().GetTimeout())
	require.NoError(t, cfg.GetSkill().Verify())
	require.NoError(t, cfg.GetServiceWriter().Verify())
}

func TestSkillEnabledRequiresAddresses(t *testing.T) {
	cfg := NewDefaultConfiguration([]string{"127.0.0.1:8091"})
	cfg.GetSkill().(*SkillConfigImpl).SetEnable(true)
	require.Error(t, cfg.GetSkill().Verify())

	cfg.GetSkill().(*SkillConfigImpl).SetAddresses([]string{"127.0.0.1:8094"})
	require.NoError(t, cfg.GetSkill().Verify())
}

func TestSkillDownloadTimeoutMustBePositive(t *testing.T) {
	cfg := NewDefaultConfiguration([]string{"127.0.0.1:8091"})
	cfg.GetSkill().SetDownloadTimeout(0)
	require.Error(t, cfg.GetSkill().Verify())
}

func TestServiceWriterAddressRejectsNonBaseURLComponents(t *testing.T) {
	cfg := NewDefaultConfiguration([]string{"127.0.0.1:8091"})
	writer := cfg.GetServiceWriter()
	for _, address := range []string{
		"http://127.0.0.1:8090?tenant=default",
		"http://127.0.0.1:8090?",
		"http://127.0.0.1:8090/#fragment",
		"http://127.0.0.1:8090/#",
		"http:127.0.0.1:8090",
	} {
		t.Run(address, func(t *testing.T) {
			writer.SetAddresses([]string{address})
			require.Error(t, writer.Verify())
		})
	}
	writer.SetAddresses([]string{"https://console.example.com/polaris"})
	require.NoError(t, writer.Verify())
}

func TestOptionalSkillConfigSectionsUseDefaults(t *testing.T) {
	const bindAddress = "global:\n  api:\n    bindIP: 127.0.0.1\n  serverConnector:\n    addresses:\n      - 127.0.0.1:8091\n"
	tests := map[string]string{
		"skill_null":               bindAddress + "skill: null\n",
		"skill_empty":              bindAddress + "skill:\n",
		"skill_empty_map":          bindAddress + "skill: {}\n",
		"skill_plugin_null":        bindAddress + "skill:\n  plugin: null\n",
		"service_writer_null":      bindAddress + "serviceWriter: null\n",
		"service_writer_empty":     bindAddress + "serviceWriter:\n",
		"service_writer_empty_map": bindAddress + "serviceWriter: {}\n",
		"both_null":                bindAddress + "skill: null\nserviceWriter: null\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var (
				cfg *ConfigurationImpl
				err error
			)
			require.NotPanics(t, func() {
				cfg, err = LoadConfiguration([]byte(input))
			})
			require.NoError(t, err)
			require.NotNil(t, cfg.GetSkill())
			require.NotNil(t, cfg.GetServiceWriter())
			assert.False(t, cfg.GetSkill().IsEnable())
			assert.Equal(t, DefaultSkillConnectTimeout, cfg.GetSkill().GetConnectTimeout())
			assert.Equal(t, DefaultSkillMessageTimeout, cfg.GetSkill().GetMessageTimeout())
			assert.Equal(t, DefaultSkillDownloadTimeout, cfg.GetSkill().GetDownloadTimeout())
			assert.Equal(t, DefaultServiceWriterTimeout, cfg.GetServiceWriter().GetTimeout())
			require.NoError(t, cfg.GetSkill().Verify())
			require.NoError(t, cfg.GetServiceWriter().Verify())
		})
	}
}
