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

const (
	EVENT_LOSSLESS_DELAY_REGISTER_START EventName = "LosslessDelayRegisterStart"
	EVENT_DIRECT_REGISTER               EventName = "DirectRegister"
	EVENT_LOSSLESS_REGISTER             EventName = "LosslessRegister"
	EVENT_LOSSLESS_DEREGISTER           EventName = "LosslessDeregister"
	EVENT_LOSSLESS_WARMUP_START         EventName = "LosslessWarmupStart"
	EVENT_LOSSLESS_WARMUP_END           EventName = "LosslessWarmupEnd"
)

type LosslessEvent interface {
	GetEventName() EventName
	GetEventTime() string
	GetClientId() string
	SetClientId(string)
	GetClientIp() string
	SetClientIp(string)
	GetClientType() string
	GetNamespace() string
	SetNamespace(string)
	GetService() string
	SetService(string)
	GetHost() string
	SetHost(string)
	GetPort() int
	SetPort(int)
	String() string
}

type LosslessEventImpl struct {
	EventName  EventName `json:"event_name"`
	EventTime  string    `json:"event_time"`
	ClientId   string    `json:"client_id"`
	ClientIp   string    `json:"client_ip"`
	ClientType string    `json:"client_type"`
	Namespace  string    `json:"namespace"`
	Service    string    `json:"service"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
}

func (e *LosslessEventImpl) GetEventName() EventName {
	return e.EventName
}

func (e *LosslessEventImpl) GetEventTime() string {
	return e.EventTime
}

func (e *LosslessEventImpl) GetClientId() string {
	return e.ClientId
}

func (e *LosslessEventImpl) SetClientId(clientId string) {
	e.ClientId = clientId
}

func (e *LosslessEventImpl) GetClientIp() string {
	return e.ClientIp
}

func (e *LosslessEventImpl) SetClientIp(clientIp string) {
	e.ClientIp = clientIp
}

func (e *LosslessEventImpl) GetClientType() string {
	return e.ClientType
}

func (e *LosslessEventImpl) GetNamespace() string {
	return e.Namespace
}

func (e *LosslessEventImpl) SetNamespace(namespace string) {
	e.Namespace = namespace
}

func (e *LosslessEventImpl) GetService() string {
	return e.Service
}

func (e *LosslessEventImpl) SetService(service string) {
	e.Service = service
}

func (e *LosslessEventImpl) GetHost() string {
	return e.Host
}

func (e *LosslessEventImpl) SetHost(host string) {
	e.Host = host
}

func (e *LosslessEventImpl) GetPort() int {
	return e.Port
}

func (e *LosslessEventImpl) SetPort(port int) {
	e.Port = port
}

func (e *LosslessEventImpl) String() string {
	return fmt.Sprintf("EventName: %s, EventTime: %s, ClientId: %s, ClientIp: %s, ClientType: %s, Namespace: %s, "+
		"Service: %s, Host: %s, Port: %d", e.EventName, e.EventTime, e.ClientId, e.ClientIp, e.ClientType,
		e.Namespace, e.Service, e.Host, e.Port)
}
