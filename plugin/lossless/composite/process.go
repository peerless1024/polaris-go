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

package composite

import (
	"fmt"
	"net/http"
	"time"

	"github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/model/event"
	"github.com/polarismesh/polaris-go/pkg/plugin/events"
)

func (p *LosslessController) Process() (*model.InstanceRegisterResponse, error) {
	if p.losslessInfo.IsDelayRegisterEnabled() {
		log.GetBaseLogger().Infof("[LosslessController] Process, delay register enabled")
		p.genAndRunGraceProbe()
		p.reportEvent(event.GetLosslessEvent(event.LosslessOnlineStart, p.losslessInfo))
		if err := p.delayRegisterChecker(); err != nil {
			return nil, err
		}
	}
	resp, err := p.engine.SyncRegister(p.losslessInfo.Instance)
	if err != nil {
		log.GetBaseLogger().Errorf("[LosslessController] Process, register failed, err: %v", err)
		return nil, err
	}
	p.reportEvent(event.GetLosslessEvent(event.LosslessOnlineEnd, p.losslessInfo))
	return resp, nil
}

func (p *LosslessController) genAndRunGraceProbe() {
	e := p.pluginCtx.ValueCtx.GetEngine()
	effectiveRule := p.losslessInfo
	if effectiveRule.IsReadinessProbeEnabled() || effectiveRule.IsOfflineProbeEnabled() {
		if effectiveRule.IsReadinessProbeEnabled() {
			log.GetBaseLogger().Infof("[LosslessController] Process, readiness probe enabled")
			effectiveRule.ReadinessProbe.HandlerFunc = genReadinessProbe(e, effectiveRule.Instance)
			e.GetAdmin().RegisterHandler(effectiveRule.ReadinessProbe)
		}
		if effectiveRule.IsOfflineProbeEnabled() {
			log.GetBaseLogger().Infof("[LosslessController] Process, offline probe enabled")
			effectiveRule.OfflineProbe.HandlerFunc = genPreStopProbe(e, effectiveRule.Instance)
			e.GetAdmin().RegisterHandler(effectiveRule.OfflineProbe)
		}
		// 启动无损上下线接口
		go e.GetAdmin().Run()
	}
}

func (p *LosslessController) delayRegisterChecker() error {
	port := p.losslessInfo.Instance.Port
	switch p.losslessInfo.DelayRegisterConfig.Strategy {
	case model.LosslessDelayRegisterStrategyDelayByTime:
		time.Sleep(p.losslessInfo.DelayRegisterConfig.DelayRegisterInterval)
		log.GetBaseLogger().Infof("[LosslessController] DelayRegisterChecker, delay register checker finished by "+
			"time:%v(second)", p.losslessInfo.DelayRegisterConfig.DelayRegisterInterval)
		return nil
	case model.LosslessDelayRegisterStrategyDelayByHealthCheck:
		// 循环进行健康检查，直到成功
		times := 0
		for {
			if times > p.pluginCfg.HealthCheckMaxRetry {
				log.GetBaseLogger().Errorf("[LosslessController] DelayRegisterChecker, health check retry times "+
					"exceeded: %v", times)
				return fmt.Errorf("health check retry times exceeded")
			}
			times++
			pass, err := doHealthCheck(port, p.losslessInfo.DelayRegisterConfig.HealthCheckConfig)
			if err != nil {
				log.GetBaseLogger().Errorf("[LosslessController] DelayRegisterChecker, health check failed, err: %v", err)
				return err
			}
			if pass {
				log.GetBaseLogger().Infof("[LosslessController] DelayRegisterChecker, health check success, " +
					"start to do register")
				return nil
			}
			log.GetBaseLogger().Infof("[LosslessController] DelayRegisterChecker, health check failed, " +
				"wait for next check")
			// 健康检查失败，等待下一个检查间隔后重试
			time.Sleep(p.losslessInfo.DelayRegisterConfig.HealthCheckConfig.HealthCheckInterval)
		}
	default:
		log.GetBaseLogger().Errorf("[LosslessController] DelayRegisterChecker, delay register strategy is not " +
			"supported, skip delay register checker")
		return fmt.Errorf("delay register strategy is not supported")
	}
}

func genReadinessProbe(e model.Engine, instance *model.InstanceRegisterRequest) func(w http.
	ResponseWriter, r *http.Request) {
	HandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		if e.GetRegisterState().IsRegistered(instance) {
			log.GetBaseLogger().Infof("[Lossless Event] losslessReadinessCheck is registered")
			w.WriteHeader(http.StatusOK)
		} else {
			log.GetBaseLogger().Infof("[Lossless Event] losslessReadinessCheck is not registered")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
	return HandlerFunc
}

func genPreStopProbe(e model.Engine, instance *model.InstanceRegisterRequest) func(w http.
	ResponseWriter, r *http.Request) {
	HandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		deregisterReq := registerToDeregister(instance)
		eventChain, ok := e.GetEventReportChain().([]events.EventReporter)
		if ok {
			events.ReportEvent(eventChain, event.GetInstanceEvent(event.LosslessOfflineStart, deregisterReq))
		} else {
			log.GetBaseLogger().Errorf("[Lossless Event] GetEventReportChain type assertion failed")
		}
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
