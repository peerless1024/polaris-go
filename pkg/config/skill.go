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
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/plugin/common"
)

const (
	// DefaultSkillConnectTimeout 是连接 Skill gRPC 服务的默认超时。
	DefaultSkillConnectTimeout = time.Second
	// DefaultSkillMessageTimeout 是 Skill gRPC 请求的默认超时。
	DefaultSkillMessageTimeout = 5 * time.Second
	// DefaultSkillDownloadTimeout 是 Skill ZIP 下载的默认超时，与 polaris-java 保持一致。
	DefaultSkillDownloadTimeout = 30 * time.Second
	// DefaultServiceWriterTimeout 是 Console HTTP 请求的默认超时。
	DefaultServiceWriterTimeout = 5 * time.Second
)

// SkillConfig 是独立 Skill Registry gRPC 服务配置。
type SkillConfig interface {
	BaseConfig
	PluginConfig
	// IsEnable 是否启用 Skill Registry 客户端。
	IsEnable() bool
	// SetEnable 设置是否启用。
	SetEnable(bool)
	// GetAddresses 获取 Skill gRPC 地址。
	GetAddresses() []string
	// SetAddresses 设置 Skill gRPC 地址。
	SetAddresses([]string)
	// GetToken 获取鉴权 token。
	GetToken() string
	// SetToken 设置鉴权 token。
	SetToken(string)
	// GetConnectTimeout 获取连接超时。
	GetConnectTimeout() time.Duration
	// SetConnectTimeout 设置连接超时。
	SetConnectTimeout(time.Duration)
	// GetMessageTimeout 获取请求超时。
	GetMessageTimeout() time.Duration
	// SetMessageTimeout 设置请求超时。
	SetMessageTimeout(time.Duration)
	// GetDownloadTimeout 获取 Skill 包下载超时。
	GetDownloadTimeout() time.Duration
	// SetDownloadTimeout 设置 Skill 包下载超时。
	SetDownloadTimeout(time.Duration)
}

// SkillConfigImpl 是 SkillConfig 的默认实现。
type SkillConfigImpl struct {
	Enable          *bool          `yaml:"enable" json:"enable"`
	Addresses       []string       `yaml:"addresses" json:"addresses"`
	Token           string         `yaml:"token" json:"token"`
	ConnectTimeout  *time.Duration `yaml:"connectTimeout" json:"connectTimeout"`
	MessageTimeout  *time.Duration `yaml:"messageTimeout" json:"messageTimeout"`
	DownloadTimeout *time.Duration `yaml:"downloadTimeout" json:"downloadTimeout"`
	Plugin          PluginConfigs  `yaml:"plugin" json:"plugin"`
}

// Init 初始化配置。
func (c *SkillConfigImpl) Init() {
	c.Plugin = PluginConfigs{}
	c.Plugin.Init(common.TypeSkillConnector)
}

// SetDefault 设置默认值。
func (c *SkillConfigImpl) SetDefault() {
	if c.Enable == nil {
		c.Enable = model.ToBoolPtr(false)
	}
	if c.ConnectTimeout == nil {
		c.ConnectTimeout = model.ToDurationPtr(DefaultSkillConnectTimeout)
	}
	if c.MessageTimeout == nil {
		c.MessageTimeout = model.ToDurationPtr(DefaultSkillMessageTimeout)
	}
	if c.DownloadTimeout == nil {
		c.DownloadTimeout = model.ToDurationPtr(DefaultSkillDownloadTimeout)
	}
	if c.Plugin == nil {
		c.Plugin = PluginConfigs{}
		c.Plugin.Init(common.TypeSkillConnector)
	}
	c.Plugin.SetDefault(common.TypeSkillConnector)
}

// Verify 校验配置。
func (c *SkillConfigImpl) Verify() error {
	if c == nil {
		return fmt.Errorf("skill config is nil")
	}
	if c.ConnectTimeout == nil || *c.ConnectTimeout <= 0 {
		return fmt.Errorf("skill.connectTimeout must be positive")
	}
	if c.MessageTimeout == nil || *c.MessageTimeout <= 0 {
		return fmt.Errorf("skill.messageTimeout must be positive")
	}
	if c.DownloadTimeout == nil || *c.DownloadTimeout <= 0 {
		return fmt.Errorf("skill.downloadTimeout must be positive")
	}
	if c.IsEnable() && len(c.Addresses) == 0 {
		return fmt.Errorf("skill.addresses must not be empty when skill.enable is true")
	}
	var errs error
	for _, address := range c.Addresses {
		if address == "" {
			errs = multierror.Append(errs, fmt.Errorf("skill.addresses contains an empty value"))
			continue
		}
		if strings.TrimSpace(address) != address {
			errs = multierror.Append(errs,
				fmt.Errorf("skill address %q must not contain surrounding whitespace", address))
			continue
		}
		host, portValue, err := net.SplitHostPort(strings.TrimSpace(address))
		if err != nil || strings.TrimSpace(host) == "" {
			errs = multierror.Append(errs,
				fmt.Errorf("skill address %q must use host:port format", address))
			continue
		}
		port, portErr := strconv.Atoi(portValue)
		if portErr != nil || port <= 0 || port > 65535 {
			errs = multierror.Append(errs,
				fmt.Errorf("skill address %q has invalid port %q", address, portValue))
		}
	}
	if err := c.Plugin.Verify(); err != nil {
		errs = multierror.Append(errs, err)
	}
	return errs
}

// GetPluginConfig 获取 Skill Connector 插件配置。
func (c *SkillConfigImpl) GetPluginConfig(pluginName string) BaseConfig {
	cfgValue, ok := c.Plugin[pluginName]
	if !ok {
		return nil
	}
	return cfgValue.(BaseConfig)
}

// SetPluginConfig 设置 Skill Connector 插件配置。
func (c *SkillConfigImpl) SetPluginConfig(pluginName string, value BaseConfig) error {
	return c.Plugin.SetPluginConfig(common.TypeSkillConnector, pluginName, value)
}

// IsEnable 是否启用 Skill Registry 客户端。
func (c *SkillConfigImpl) IsEnable() bool {
	return c != nil && c.Enable != nil && *c.Enable
}

// SetEnable 设置是否启用。
func (c *SkillConfigImpl) SetEnable(enable bool) {
	c.Enable = model.ToBoolPtr(enable)
}

// GetAddresses 获取 Skill gRPC 地址。
func (c *SkillConfigImpl) GetAddresses() []string {
	if c == nil {
		return nil
	}
	return c.Addresses
}

// SetAddresses 设置 Skill gRPC 地址。
func (c *SkillConfigImpl) SetAddresses(addresses []string) {
	c.Addresses = addresses
}

// GetToken 获取鉴权 token。
func (c *SkillConfigImpl) GetToken() string {
	if c == nil {
		return ""
	}
	return c.Token
}

// SetToken 设置鉴权 token。
func (c *SkillConfigImpl) SetToken(token string) {
	c.Token = token
}

// GetConnectTimeout 获取连接超时。
func (c *SkillConfigImpl) GetConnectTimeout() time.Duration {
	return *c.ConnectTimeout
}

// SetConnectTimeout 设置连接超时。
func (c *SkillConfigImpl) SetConnectTimeout(timeout time.Duration) {
	c.ConnectTimeout = model.ToDurationPtr(timeout)
}

// GetMessageTimeout 获取请求超时。
func (c *SkillConfigImpl) GetMessageTimeout() time.Duration {
	return *c.MessageTimeout
}

// SetMessageTimeout 设置请求超时。
func (c *SkillConfigImpl) SetMessageTimeout(timeout time.Duration) {
	c.MessageTimeout = model.ToDurationPtr(timeout)
}

// GetDownloadTimeout 获取 Skill 包下载超时。
func (c *SkillConfigImpl) GetDownloadTimeout() time.Duration {
	if c == nil || c.DownloadTimeout == nil {
		return DefaultSkillDownloadTimeout
	}
	return *c.DownloadTimeout
}

// SetDownloadTimeout 设置 Skill 包下载超时。
func (c *SkillConfigImpl) SetDownloadTimeout(timeout time.Duration) {
	c.DownloadTimeout = model.ToDurationPtr(timeout)
}
