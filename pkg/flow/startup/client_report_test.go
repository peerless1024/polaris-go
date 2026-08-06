/**
 * Tencent is pleased to support the open source community by making polaris-go available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package startup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	configflow "github.com/polarismesh/polaris-go/pkg/flow/configuration"
)

// TestConfigMetadataPayload_Marshal 验证 ReportClient 上报 config_metadata 的 JSON 结构，
// 确认顶层 kind/config_watch 与数组元素的 snake_case 字段名（与服务端配置中心三级树命名一致）。
func TestConfigMetadataPayload_Marshal(t *testing.T) {
	payload := configMetadataPayload{
		Kind: "config",
		ConfigWatch: []configflow.ConfigFileMetadataItem{
			{Namespace: "default", Group: "g1", FileName: "f1", Version: 3, Md5: "md5_1"},
			{Namespace: "default", Group: "g2", FileName: "f2", Version: 5, Md5: ""},
		},
	}
	data, err := json.Marshal(payload)
	assert.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"kind":"config"`)
	assert.Contains(t, s, `"config_watch":[`)
	// snake_case 字段
	assert.Contains(t, s, `"file_name":"f1"`)
	assert.Contains(t, s, `"namespace":"default"`)
	assert.Contains(t, s, `"group":"g1"`)
	assert.Contains(t, s, `"version":3`)
	assert.Contains(t, s, `"md5":"md5_1"`)
}

// TestConfigMetadataPayload_EmptyWatch 验证无监听文件时 JSON 仍含 kind 与空 config_watch 数组。
func TestConfigMetadataPayload_EmptyWatch(t *testing.T) {
	payload := configMetadataPayload{Kind: "config", ConfigWatch: []configflow.ConfigFileMetadataItem{}}
	data, err := json.Marshal(payload)
	assert.NoError(t, err)
	assert.Equal(t, `{"kind":"config","config_watch":[]}`, string(data))
}

// TestClientInfoFileName_UniquePerClientID 验证持久化文件名按 clientID 隔离：
// 同进程多 context（clientID 不同）得到不同文件名，避免 client_info.json 互相覆盖。
func TestClientInfoFileName_UniquePerClientID(t *testing.T) {
	f1 := clientInfoFileName("host-1234-0")
	f2 := clientInfoFileName("host-1234-1")
	f3 := clientInfoFileName("pod-xyz-0")
	assert.Equal(t, "client_info_host-1234-0.json", f1)
	assert.Equal(t, "client_info_host-1234-1.json", f2)
	assert.Equal(t, "client_info_pod-xyz-0.json", f3)
	assert.NotEqual(t, f1, f2, "同进程多 context 文件名应不同")
	assert.NotEqual(t, f1, f3, "不同 clientID 文件名应不同")
}

// TestClientInfoFileName_Format 验证文件名格式与扩展名。
func TestClientInfoFileName_Format(t *testing.T) {
	name := clientInfoFileName("c1")
	assert.Equal(t, "client_info_c1.json", name)
	assert.True(t, strings.HasSuffix(name, ".json"), "应以 .json 结尾")
	assert.True(t, strings.HasPrefix(name, "client_info_"), "应以 client_info_ 开头")
}
