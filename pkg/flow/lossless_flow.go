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

package flow

import (
	"fmt"
	"net/http"
	"time"

	"github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/model/event"
	"github.com/polarismesh/polaris-go/pkg/plugin/events"
)

// SyncLosslessRegister 同步进行服务注册
func (e *Engine) SyncLosslessRegister(instance *model.InstanceRegisterRequest) (*model.InstanceRegisterResponse,
	error) {
	losslessRule, err := e.SyncGetServiceRule(model.EventLosslessRule, &model.GetServiceRuleRequest{
		Namespace: instance.Namespace,
		Service:   instance.Service,
	})
	if err != nil {
		log.GetBaseLogger().Errorf("[Lossless Event] SyncLosslessRegister SyncGetServiceRule error: %v", err)
		return nil, err
	}
	log.GetBaseLogger().Infof("[Lossless Event] SyncLosslessRegister SyncGetServiceRule success: %v", losslessRule)
	// 当lossless为nil时, 说明本地未开启无损上下线功能插件, 直接注册
	if e.lossless == nil || losslessRule == nil || losslessRule.Value == nil {
		log.GetBaseLogger().Infof("[Lossless Event] SyncLosslessRegister lossless is not enable, register directly")
		return e.SyncRegister(instance)
	}
	effectiveRule := e.lossless.OnPreProcess(losslessRule)
	if effectiveRule == nil {
		err = fmt.Errorf("SyncLosslessRegister OnPreProcess return nil")
		log.GetBaseLogger().Errorf("[Lossless Event] SyncLosslessRegister OnPreProcess error: %v", err)
		return nil, err
	}
	if effectiveRule.ReadinessProbe != nil || effectiveRule.OfflineProbe != nil {
		if effectiveRule.ReadinessProbe != nil {
			effectiveRule.ReadinessProbe.HandlerFunc = e.losslessReadinessCheck()
			e.admin.RegisterHandler(effectiveRule.ReadinessProbe)
		}
		if effectiveRule.OfflineProbe != nil {
			effectiveRule.OfflineProbe.HandlerFunc = e.losslessOfflineProcess(instance)
			e.admin.RegisterHandler(effectiveRule.OfflineProbe)
		}
		// 启动无损上下线接口
		go e.admin.Run()
	}
	if !effectiveRule.IsDelayRegisterEnabled() {
		log.GetBaseLogger().Infof("[Lossless Event] SyncLosslessRegister delayRegisterEnabled is false, register directly")
		return e.SyncRegister(instance)
	}
	// 上报无损上下线事件, 无损上线开始
	events.ReportEvent(e.eventChain, event.GetLosslessEvent(event.LosslessOnlineStart, instance, effectiveRule))
	// 延迟注册检查
	err = e.lossless.DelayRegisterChecker(instance.Port)
	if err != nil {
		log.GetBaseLogger().Errorf("[Lossless Event] SyncLosslessRegister DelayRegisterChecker error: %v", err)
		return nil, err
	}
	resp, err := e.SyncRegister(instance)
	if err != nil {
		log.GetBaseLogger().Errorf("[Lossless Event] SyncLosslessRegister SyncRegister error: %v", err)
		return resp, err
	}
	// 上报无损上下线事件, 无损上线结束
	events.ReportEvent(e.eventChain, event.GetLosslessEvent(event.LosslessOnlineEnd, instance, effectiveRule))
	if effectiveRule.IsWarmUpEnabled() {
		go func() {
			events.ReportEvent(e.eventChain, event.GetLosslessEvent(event.LosslessWarmupStart, instance, effectiveRule))
			time.Sleep(effectiveRule.WarmUpConfig.Interval)
			events.ReportEvent(e.eventChain, event.GetLosslessEvent(event.LosslessWarmupEnd, instance, effectiveRule))
		}()
	}
	return resp, nil
}

// 是否改成函数
func (e *Engine) losslessReadinessCheck() func(w http.ResponseWriter, r *http.Request) {
	HandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// TODO 获取注册状态
		w.WriteHeader(http.StatusOK)
	}
	return HandlerFunc
}

// 是否改成函数
func (e *Engine) losslessOfflineProcess(instance *model.InstanceRegisterRequest) func(w http.ResponseWriter, r *http.Request) {
	HandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		deregisterReq := registerToDeregister(instance)
		events.ReportEvent(e.eventChain, event.GetInstanceEvent(event.LosslessOfflineStart, deregisterReq))
		if err := e.SyncDeregister(deregisterReq); err == nil {
			log.GetBaseLogger().Infof("[Lossless Event] losslessOfflineProcess SyncDeregister success")
			w.WriteHeader(http.StatusOK)
		} else {
			log.GetBaseLogger().Errorf("[Lossless Event] losslessOfflineProcess SyncDeregister error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	return HandlerFunc
}

func registerToDeregister(instance *model.InstanceRegisterRequest) *model.InstanceDeRegisterRequest {
	return &model.InstanceDeRegisterRequest{
		Namespace:    instance.Namespace,
		Service:      instance.Service,
		Host:         instance.Host,
		Port:         instance.Port,
		InstanceID:   instance.InstanceId,
		ServiceToken: instance.ServiceToken,
		Timeout:      instance.Timeout,
		RetryCount:   instance.RetryCount,
	}
}
