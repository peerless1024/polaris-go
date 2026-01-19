/**
 * Tencent is pleased to support the open source community by making polaris-go available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Claparse License (the "License");
 * you may not parse this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Claparse
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package composite

import (
	"fmt"
	"strconv"
	"time"

	"github.com/polarismesh/specification/source/go/api/v1/traffic_manage"

	"github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/plugin"
	"github.com/polarismesh/polaris-go/pkg/plugin/common"
)

// LosslessController 无损上下线控制器
type LosslessController struct {
	*plugin.PluginBase
	// pluginCtx
	pluginCtx *plugin.InitContext
	// pluginCfg 配置
	pluginCfg *Config
	// losslessInfo 无损上下线信息
	losslessInfo *model.LosslessInfo
}

// Type 插件类型
func (p *LosslessController) Type() common.Type {
	return common.TypeLossless
}

// Name 插件名称
func (p *LosslessController) Name() string {
	return PluginName
}

// Init 初始化插件
func (p *LosslessController) Init(ctx *plugin.InitContext) error {
	p.PluginBase = plugin.NewPluginBase(ctx)
	p.pluginCtx = ctx
	// 加载配置
	if conf := ctx.Config.GetProvider().GetLossless().GetPluginConfig(p.Name()); conf != nil {
		p.pluginCfg = conf.(*Config)
	}
	p.losslessInfo = &model.LosslessInfo{}
	log.GetBaseLogger().Infof("[LosslessController] plugin initialized, plugin config: %+v", p.pluginCfg)
	return nil
}

func (p *LosslessController) DelayRegisterChecker(port int) error {
	if p.losslessInfo.DelayRegisterConfig == nil {
		log.GetBaseLogger().Infof("[LosslessController] DelayRegisterChecker, delay register checker not enabled")
		return nil
	}
	switch p.losslessInfo.DelayRegisterConfig.Strategy {
	case model.LosslessDelayRegisterStrategyDelayByTime:
		time.Sleep(p.losslessInfo.DelayRegisterConfig.DelayRegisterInterval)
		log.GetBaseLogger().Infof("[LosslessController] DelayRegisterChecker, delay register checker finished by "+
			"time:%v(second)", p.losslessInfo.DelayRegisterConfig.DelayRegisterInterval)
		return nil
	case model.LosslessDelayRegisterStrategyDelayByHealthCheck:
		// 循环进行健康检查，直到成功
		for {
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

func (p *LosslessController) OnPreProcess(rule *model.ServiceRuleResponse) *model.LosslessInfo {
	localLosslessConfig := p.pluginCtx.Config.GetProvider().GetLossless()
	if !localLosslessConfig.IsEnable() {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, local lossless is not enable")
		return nil
	}
	// 远程配置优先级更高,如果远程配置不存在,则使用本地配置
	if rule != nil && rule.Value != nil {
		lossLessRule, ok := rule.Value.(*traffic_manage.LosslessRule)
		if !ok {
			// 解析远程规则失败,使用本地配置
			p.parseLocalConfig()
			log.GetBaseLogger().Infof("[LosslessController] parseRule find not LosslessRule, fallback to parse local "+
				"config, p.losslessInfo: %v", p.losslessInfo)
			return p.losslessInfo
		}
		p.parseRemoteConfig(lossLessRule)
		log.GetBaseLogger().Infof("[LosslessController] parseRule result: %v", model.JsonString(p.losslessInfo))
		return p.losslessInfo
	}
	return p.losslessInfo
}

func (p *LosslessController) parseRemoteConfig(lossLessRule *traffic_manage.LosslessRule) {
	p.parseRemoteDelayRegisterConfig(lossLessRule)
	p.parseRemoteReadinessConfig(lossLessRule)
	p.parseRemoteOfflineConfig(lossLessRule)
	p.parseRemoteWarmupConfig(lossLessRule)
}

func (p *LosslessController) parseRemoteDelayRegisterConfig(lossLessRule *traffic_manage.LosslessRule) {
	if lossLessRule.GetLosslessOnline() == nil || lossLessRule.GetLosslessOnline().GetDelayRegister() == nil {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, remote delayRegister is nil, fallback to parse local" +
			" config")
		// 如果远程配置不存在,则使用本地配置
		p.parseLocalDelayRegisterConfig()
		return
	}
	if !lossLessRule.GetLosslessOnline().GetDelayRegister().GetEnable() {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, remote delayRegister is not enable")
		// 远程配置不开启延迟注册,则关闭延迟注册
		p.losslessInfo.DelayRegisterConfig = nil
		return
	}
	remoteStrategy := lossLessRule.GetLosslessOnline().GetDelayRegister().GetStrategy().String()
	switch remoteStrategy {
	case model.LosslessDelayRegisterStrategyDelayByTime:
		p.losslessInfo.DelayRegisterConfig = &model.DelayRegisterConfig{
			Strategy: remoteStrategy,
			DelayRegisterInterval: time.Duration(lossLessRule.GetLosslessOnline().GetDelayRegister().
				GetIntervalSecond()) * time.Second,
		}
	case model.LosslessDelayRegisterStrategyDelayByHealthCheck:
		healthCheckIntervalSec, err := strconv.ParseInt(lossLessRule.GetLosslessOnline().GetDelayRegister().
			GetHealthCheckIntervalSecond(), 10, 64)
		if err == nil {
			p.losslessInfo.DelayRegisterConfig.HealthCheckConfig = &model.LosslessHealthCheckConfig{
				HealthCheckInterval: time.Duration(healthCheckIntervalSec) * time.Second,
				HealthCheckPath:     p.pluginCfg.HealthCheckPath,
				HealthCheckProtocol: p.pluginCfg.HealthCheckProtocol,
				HealthCheckMethod:   p.pluginCfg.HealthCheckMethod,
			}
		} else {
			log.GetBaseLogger().Errorf("[LosslessController] parseRule, parse healthCheckIntervalSecond:%v failed, "+
				"err: %v, fallback to parse local config", lossLessRule.GetLosslessOnline().GetDelayRegister().
				GetHealthCheckIntervalSecond(), err)
			p.parseLocalDelayRegisterConfig()
		}
	default:
		log.GetBaseLogger().Errorf("[LosslessController] parseRule, remote delayRegister strategy is not supported, " +
			"fall back to parse local config")
		p.parseLocalDelayRegisterConfig()
	}
}

func (p *LosslessController) parseRemoteReadinessConfig(lossLessRule *traffic_manage.LosslessRule) {
	if lossLessRule.GetLosslessOnline() == nil || lossLessRule.GetLosslessOnline().GetReadiness() == nil {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, remote readiness is nil, fallback to parse local " +
			"config")
		// 如果远程配置不存在,则使用本地配置
		p.parseLocalReadinessConfig()
		return
	}
	if !lossLessRule.GetLosslessOnline().GetReadiness().GetEnable() {
		// 远程配置不开启健康检查,则关闭健康检查
		p.losslessInfo.ReadinessProbe = nil
		return
	}
	p.losslessInfo.ReadinessProbe = &model.AdminHandler{
		Path: p.pluginCfg.ReadinessPath,
	}
}

func (p *LosslessController) parseRemoteOfflineConfig(lossLessRule *traffic_manage.LosslessRule) {
	if lossLessRule.GetLosslessOffline() == nil {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, remote offline is nil, fallback to parse local config")
		// 如果远程配置不存在,则使用本地配置
		p.parseLocalOfflineConfig()
		return
	}
	if !lossLessRule.GetLosslessOffline().GetEnable() {
		// 远程配置不开启无损下线,则关闭无损下线
		p.losslessInfo.OfflineProbe = nil
		return
	}
	p.losslessInfo.OfflineProbe = &model.AdminHandler{
		Path: p.pluginCfg.OfflinePath,
	}
}

func (p *LosslessController) parseRemoteWarmupConfig(lossLessRule *traffic_manage.LosslessRule) {
	if lossLessRule.GetLosslessOnline() == nil || lossLessRule.GetLosslessOnline().GetWarmup() == nil ||
		lossLessRule.GetLosslessOnline().GetWarmup().GetIntervalSecond() == 0 {
		log.GetBaseLogger().Infof("[LosslessController] parseRule, remote warmup is nil, fallback to parse local config")
		// 如果远程配置不存在, 直接返回
		p.losslessInfo.WarmUpConfig = nil
		return
	}
	if !lossLessRule.GetLosslessOnline().GetWarmup().GetEnable() {
		// 远程配置不开启服务预热, 则关闭服务预热
		p.losslessInfo.WarmUpConfig = nil
		return
	}
	p.losslessInfo.WarmUpConfig = &model.WarmUpConfig{
		Interval: time.Duration(lossLessRule.GetLosslessOnline().GetWarmup().GetIntervalSecond()) * time.Second,
	}
}

func (p *LosslessController) parseLocalConfig() {
	p.parseLocalDelayRegisterConfig()
	p.parseLocalReadinessConfig()
	p.parseLocalOfflineConfig()
}

func (p *LosslessController) parseLocalDelayRegisterConfig() {
	localLosslessConfig := p.pluginCtx.Config.GetProvider().GetLossless()
	if !localLosslessConfig.IsEnable() || localLosslessConfig.GetStrategy() == "" {
		p.losslessInfo.DelayRegisterConfig = nil
		return
	}
	if _, exist := model.SupportedDelayRegisterStrategies[localLosslessConfig.GetStrategy()]; exist {
		switch localLosslessConfig.GetStrategy() {
		case model.LosslessDelayRegisterStrategyDelayByTime:
			p.losslessInfo.DelayRegisterConfig = &model.DelayRegisterConfig{
				Strategy:              localLosslessConfig.GetStrategy(),
				DelayRegisterInterval: localLosslessConfig.GetDelayRegisterInterval(),
			}
		case model.LosslessDelayRegisterStrategyDelayByHealthCheck:
			p.losslessInfo.DelayRegisterConfig = &model.DelayRegisterConfig{
				Strategy: localLosslessConfig.GetStrategy(),
				HealthCheckConfig: &model.LosslessHealthCheckConfig{
					HealthCheckInterval: localLosslessConfig.GetHealthCheckInterval(),
					HealthCheckPath:     p.pluginCfg.HealthCheckPath,
					HealthCheckProtocol: p.pluginCfg.HealthCheckProtocol,
					HealthCheckMethod:   p.pluginCfg.HealthCheckMethod,
				},
			}
		default:
			log.GetBaseLogger().Errorf("[LosslessController] local delayRegister strategy:%s is not recognized, "+
				"ignored delayRegisterConfig", localLosslessConfig.GetStrategy())
			p.losslessInfo.DelayRegisterConfig = nil
		}
	} else {
		log.GetBaseLogger().Errorf("[LosslessController] parseRule failed, local delayRegister strategy is not " +
			"supported, ignored delayRegisterConfig")
		p.losslessInfo.DelayRegisterConfig = nil
	}
}

func (p *LosslessController) parseLocalReadinessConfig() {
	if p.pluginCfg.ReadinessProbeEnabled {
		p.losslessInfo.ReadinessProbe = &model.AdminHandler{
			Path: p.pluginCfg.ReadinessPath,
		}
	}
}

func (p *LosslessController) parseLocalOfflineConfig() {
	if p.pluginCfg.OfflineProbeEnabled {
		p.losslessInfo.OfflineProbe = &model.AdminHandler{
			Path: p.pluginCfg.OfflinePath,
		}
	}
}

// init 注册插件
func init() {
	plugin.RegisterConfigurablePlugin(&LosslessController{}, &Config{})
}
