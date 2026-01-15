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

package model

import (
	"fmt"
	"sync"
	"time"
)

const (
	// LosslessDelayRegisterStrategyDelayByTime 时长延迟注册策略
	LosslessDelayRegisterStrategyDelayByTime = "DELAY_BY_TIME"
	// LosslessDelayRegisterStrategyDelayByHealthCheck 探测延迟注册策略
	LosslessDelayRegisterStrategyDelayByHealthCheck = "DELAY_BY_HEALTH_CHECK"
)

var SupportedDelayRegisterStrategies = map[string]struct{}{
	LosslessDelayRegisterStrategyDelayByTime:        {},
	LosslessDelayRegisterStrategyDelayByHealthCheck: {},
}

type LosslessInfo struct {
	LocalEnabled                bool
	DelayRegisterEnabled        bool
	ReadinessProbeEnabled       bool
	OfflineProbeEnabled         bool
	LosslessDelayRegisterConfig *LosslessDelayRegisterConfig
	ReadinessProbe              *AdminHandler
	OfflineProbe                *AdminHandler
	RegisterStatuses            sync.Map
}

func (c *LosslessInfo) String() string {
	return fmt.Sprintf("LosslessInfo{LocalEnabled=%t, DelayRegisterEnabled=%t, ReadinessProbeEnabled=%t, "+
		"OfflineProbeEnabled=%t, LosslessDelayRegisterConfig=%s, ReadinessProbePath=%s, OfflineProbePath=%s}",
		c.LocalEnabled, c.DelayRegisterEnabled, c.ReadinessProbeEnabled, c.OfflineProbeEnabled,
		c.LosslessDelayRegisterConfig, c.ReadinessProbe, c.OfflineProbe)
}

type LosslessDelayRegisterConfig struct {
	Strategy              string
	DelayRegisterInterval time.Duration
	HealthCheckConfig     *LosslessHealthCheckConfig
}

func (c *LosslessDelayRegisterConfig) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("LosslessDelayRegisterConfig{Strategy=%s, DelayRegisterInterval=%v, HealthCheckConfig=%s}",
		c.Strategy, c.DelayRegisterInterval, c.HealthCheckConfig)
}

type LosslessHealthCheckConfig struct {
	HealthCheckInterval time.Duration
	HealthCheckPath     string
	HealthCheckProtocol string
	HealthCheckMethod   string
}

func (c *LosslessHealthCheckConfig) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("LosslessHealthCheckConfig{HealthCheckInterval=%v, HealthCheckPath=%s, "+
		"HealthCheckProtocol=%s, HealthCheckMethod=%s}", c.HealthCheckInterval, c.HealthCheckPath,
		c.HealthCheckProtocol, c.HealthCheckMethod)
}

type LosslessEvent struct {
}
