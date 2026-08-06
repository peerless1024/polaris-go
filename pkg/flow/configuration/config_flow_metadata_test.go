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

package configuration

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/plugin/configconnector"
)

// TestConfigFileFlow_GetWatchedConfigFileMetadata 验证从 configFilePool 收集监听文件元数据，
// 包含 namespace/group/file_name/version/md5 五项，供 ReportClient 上报 config_metadata。
func TestConfigFileFlow_GetWatchedConfigFileMetadata(t *testing.T) {
	ref1 := &atomic.Value{}
	ref1.Store(&configconnector.ConfigFile{
		Namespace: "default", FileGroup: "g1", FileName: "f1", Version: 3, Md5: "md5_1",
	})
	ref2 := &atomic.Value{}
	ref2.Store(&configconnector.ConfigFile{
		Namespace: "default", FileGroup: "g2", FileName: "f2", Version: 5, Md5: "md5_2",
	})
	flow := &ConfigFileFlow{
		configFilePool: map[string]*ConfigFileRepo{
			"k1": {
				configFileMetadata: &model.DefaultConfigFileMetadata{
					Namespace: "default", FileGroup: "g1", FileName: "f1",
				},
				remoteConfigFileRef: ref1,
			},
			"k2": {
				configFileMetadata: &model.DefaultConfigFileMetadata{
					Namespace: "default", FileGroup: "g2", FileName: "f2",
				},
				remoteConfigFileRef: ref2,
			},
		},
		notifiedVersion: map[string]uint64{"k1": 3, "k2": 5},
	}

	items := flow.GetWatchedConfigFileMetadata()
	assert.Equal(t, 2, len(items))

	byKey := make(map[string]ConfigFileMetadataItem, len(items))
	for _, it := range items {
		byKey[it.Group+"/"+it.FileName] = it
	}
	it1 := byKey["g1/f1"]
	assert.Equal(t, "default", it1.Namespace)
	assert.Equal(t, "g1", it1.Group)
	assert.Equal(t, "f1", it1.FileName)
	assert.Equal(t, uint64(3), it1.Version)
	assert.Equal(t, "md5_1", it1.Md5)
	it2 := byKey["g2/f2"]
	assert.Equal(t, "md5_2", it2.Md5)
	assert.Equal(t, uint64(5), it2.Version)
}

// TestConfigFileFlow_GetWatchedConfigFileMetadata_EmptyRemoteFile 验证 repo 尚未拉取到远端配置文件时
// loadRemoteFile 返回 nil，md5 应留空而不 panic。
func TestConfigFileFlow_GetWatchedConfigFileMetadata_EmptyRemoteFile(t *testing.T) {
	flow := &ConfigFileFlow{
		configFilePool: map[string]*ConfigFileRepo{
			"k1": {
				configFileMetadata: &model.DefaultConfigFileMetadata{
					Namespace: "default", FileGroup: "g1", FileName: "f1",
				},
				remoteConfigFileRef: &atomic.Value{}, // 未 Store，loadRemoteFile 返回 nil
			},
		},
		notifiedVersion: map[string]uint64{"k1": 0},
	}
	items := flow.GetWatchedConfigFileMetadata()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "", items[0].Md5)
	assert.Equal(t, "default", items[0].Namespace)
	assert.Equal(t, "f1", items[0].FileName)
}

// TestConfigFileFlow_GetWatchedConfigFileMetadata_EmptyPool 验证空池返回非 nil 空切片。
func TestConfigFileFlow_GetWatchedConfigFileMetadata_EmptyPool(t *testing.T) {
	flow := &ConfigFileFlow{
		configFilePool:  map[string]*ConfigFileRepo{},
		notifiedVersion: map[string]uint64{},
	}
	items := flow.GetWatchedConfigFileMetadata()
	assert.NotNil(t, items)
	assert.Equal(t, 0, len(items))
}
