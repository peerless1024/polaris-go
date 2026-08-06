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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"

	"github.com/polarismesh/polaris-go/pkg/config"
	configflow "github.com/polarismesh/polaris-go/pkg/flow/configuration"
	"github.com/polarismesh/polaris-go/pkg/flow/data"
	"github.com/polarismesh/polaris-go/pkg/log"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/polarismesh/polaris-go/pkg/plugin"
	"github.com/polarismesh/polaris-go/pkg/plugin/localregistry"
	statreporter "github.com/polarismesh/polaris-go/pkg/plugin/metrics"
	"github.com/polarismesh/polaris-go/pkg/plugin/serverconnector"
	"github.com/polarismesh/polaris-go/pkg/sdk"
	"github.com/polarismesh/polaris-go/pkg/version"
)

// NewReportClientCallBack 创建上报回调。
// configFlow 为配置文件流，非 nil 时用于收集监听配置文件元数据上报；配置中心未启用时传 nil。
func NewReportClientCallBack(
	cfg config.Configuration, supplier plugin.Supplier, globalCtx sdk.ValueContext,
	configFlow *configflow.ConfigFileFlow) (*ReportClientCallBack, error) {
	var err error
	var callback = &ReportClientCallBack{}
	if callback.connector, err = data.GetServerConnector(cfg, supplier); err != nil {
		return nil, err
	}
	if callback.registry, err = data.GetRegistry(cfg, supplier); err != nil {
		return nil, err
	}
	if callback.reporterChain, err = data.GetStatReporterChain(cfg, supplier); err != nil {
		return nil, err
	}
	callback.configuration = cfg
	callback.globalCtx = globalCtx
	callback.configFlow = configFlow
	callback.interval = cfg.GetGlobal().GetAPI().GetReportInterval()
	callback.logCtx = globalCtx.GetContextLogger()
	callback.loadLocalClientReportResult()
	return callback, nil
}

// ReportClientCallBack 上报客户端状态任务回调
type ReportClientCallBack struct {
	connector     serverconnector.ServerConnector
	registry      localregistry.InstancesRegistry
	configuration config.Configuration
	globalCtx     sdk.ValueContext
	// configFlow 配置文件流，配置中心未启用时为 nil
	configFlow    *configflow.ConfigFileFlow
	interval      time.Duration
	reporterChain []statreporter.StatReporter
	logCtx        *log.ContextLogger
	// lastLocation 记录上次成功持久化的地域信息，用于对比判断是否需要重新写入 client_info.json
	lastLocation *model.Location
	// reportPending 标记是否已有待执行的即时上报（debounce 合并用），0=无 1=有
	reportPending int32
}

const (
	// reportDebounceWindow 即时上报的合并窗口。
	// 批量订阅配置文件时，窗口内的多次 TriggerNow 合并为一次上报，避免上报风暴。
	reportDebounceWindow = 500 * time.Millisecond
)

const (
	// clientInfoPersistFilePrefix 地域信息持久化文件名前缀，
	// 实际文件名追加 clientID（如 client_info_host-1234-0.json），按 context 隔离避免互相覆盖。
	clientInfoPersistFilePrefix = "client_info"
)

// clientInfoFile 返回当前 context 的持久化文件名：client_info_<clientID>.json。
// 同进程多 context 各自写各自文件，互不覆盖；clientID 含的字符（PodName、host、pid、seq、UUID）均合法。
func (r *ReportClientCallBack) clientInfoFile() string {
	return clientInfoFileName(r.globalCtx.GetClientId())
}

// clientInfoFileName 按 clientID 拼接持久化文件名：client_info_<clientID>.json。
// 抽为包级函数便于单测，并确保文件名随 clientID 变化而唯一。
func clientInfoFileName(clientID string) string {
	return fmt.Sprintf("%s_%s.json", clientInfoPersistFilePrefix, clientID)
}

// loadLocalClientReportResult 从本地缓存加载上报结果信息
func (r *ReportClientCallBack) loadLocalClientReportResult() {
	logBase := r.logCtx.GetBaseLogger()
	resp := &apiservice.Response{}
	cachedFile := r.clientInfoFile()
	err := r.registry.LoadPersistedMessage(cachedFile, resp)
	if err != nil {
		logBase.Warnf("fail to load local region info from %s, err is %v", cachedFile, err)
		return
	}
	location := resp.GetClient().GetLocation()
	loc := &model.Location{
		Region: location.GetRegion().GetValue(),
		Zone:   location.GetZone().GetValue(),
		Campus: location.GetCampus().GetValue(),
	}
	// 初始化 lastLocation，避免首次上报时与缓存相同的 location 也触发重复写入
	r.lastLocation = loc
	r.updateLocation(loc, nil)
}

// reportClientRequest 客户端上报的请求
func (r *ReportClientCallBack) reportClientRequest() *model.ReportClientRequest {
	apiConfig := r.configuration.GetGlobal().GetAPI()
	clientHost := apiConfig.GetBindIP()
	reportClientReq := &model.ReportClientRequest{
		Version:        version.Version,
		Timeout:        r.configuration.GetGlobal().GetAPI().GetTimeout(),
		PersistHandler: r.persistHandlerWithLocationCheck,
	}
	if len(clientHost) > 0 {
		reportClientReq.Host = clientHost
	}

	infos := make([]model.StatInfo, 0, len(r.reporterChain))

	// 收集当前的所有metric插件链的元信息
	for i := range r.reporterChain {
		stat := r.reporterChain[i].Info()
		if stat.Empty() {
			continue
		}
		infos = append(infos, stat)
	}

	reportClientReq.StatInfos = infos
	reportClientReq.ID = r.globalCtx.GetClientId()
	r.fillConfigMetadata(reportClientReq)
	return reportClientReq
}

// persistHandlerWithLocationCheck 带地域信息变更检查的持久化处理函数
// 只有当服务端返回的地域信息与上次持久化的不同时，才执行写入操作，避免不必要的磁盘 I/O
func (r *ReportClientCallBack) persistHandlerWithLocationCheck(message proto.Message) error {
	cachedFile := r.clientInfoFile()
	resp, ok := message.(*apiservice.Response)
	if !ok {
		// 类型不匹配时直接持久化
		return r.registry.PersistMessage(cachedFile, message)
	}
	loc := resp.GetClient().GetLocation()
	newLocation := &model.Location{
		Region: loc.GetRegion().GetValue(),
		Zone:   loc.GetZone().GetValue(),
		Campus: loc.GetCampus().GetValue(),
	}
	// 对比新旧地域信息，相同则跳过写入
	if r.lastLocation != nil && *r.lastLocation == *newLocation {
		return nil
	}
	// 地域信息发生变化或首次写入，执行持久化
	if err := r.registry.PersistMessage(cachedFile, message); err != nil {
		return err
	}
	r.lastLocation = newLocation
	r.logCtx.GetBaseLogger().Infof("%s updated, location changed to {Region:%s, Zone:%s, Campus:%s}",
		cachedFile, newLocation.Region, newLocation.Zone, newLocation.Campus)
	return nil
}

// Process 执行任务
func (r *ReportClientCallBack) Process(
	taskKey interface{}, taskValue interface{}, lastProcessTime time.Time) model.TaskResult {
	if !lastProcessTime.IsZero() && time.Since(lastProcessTime) < r.interval {
		return model.SKIP
	}
	reportClientReq := r.reportClientRequest()
	if err := reportClientReq.Validate(); err != nil {
		r.logCtx.GetBaseLogger().Errorf("report client request fatal validate error:%v", err)
		return model.TERMINATE
	}

	reportClientResp, err := r.connector.ReportClient(reportClientReq)
	if err != nil {
		r.logCtx.GetBaseLogger().Errorf("report client info:%+v, error:%v", reportClientReq, err)
		r.updateLocation(nil, err.(model.SDKError))
		// 发生错误也要重试，直到获取到地域信息为止
		return model.CONTINUE
	}

	r.updateLocation(&model.Location{
		Region: reportClientResp.Region,
		Zone:   reportClientResp.Zone,
		Campus: reportClientResp.Campus,
	}, nil)
	return model.CONTINUE
}

// OnTaskEvent 任务事件回调
func (r *ReportClientCallBack) OnTaskEvent(event model.TaskEvent) {

}

// updateLocation 更新区域属性
func (r *ReportClientCallBack) updateLocation(location *model.Location, lastErr model.SDKError) {
	// 如果SDK设置了本地获取 location，则忽略 ReportClient 的数据
	if len(r.configuration.GetGlobal().GetLocation().GetProviders()) != 0 {
		return
	}

	if nil != location {
		// 只在地域信息首次获取或发生变化时打印日志，避免重复输出相同内容
		if r.lastLocation == nil || *r.lastLocation != *location {
			r.logCtx.GetBaseLogger().Infof("current client area info is {Region:%s, Zone:%s, Campus:%s}",
				location.Region, location.Zone, location.Campus)
		}
	}
	if r.globalCtx.SetCurrentLocation(location, lastErr) {
		r.logCtx.GetBaseLogger().Infof("client area info is ready")
	}
}

// fillConfigMetadata 填充配置中心启用状态与监听文件列表元数据到上报请求。
// ConfigEnabled 取配置中心是否启用；启用且 configFlow 非 nil 时序列化监听文件列表为 config_metadata JSON。
// req 不能为 nil；配置中心未启用或 configFlow 为 nil 时仅设置 ConfigEnabled，ConfigMetadata 留空。
func (r *ReportClientCallBack) fillConfigMetadata(req *model.ReportClientRequest) {
	req.ConfigEnabled = r.configuration.GetConfigFile().IsEnable()
	if !req.ConfigEnabled || r.configFlow == nil {
		return
	}
	items := r.configFlow.GetWatchedConfigFileMetadata()
	payload := configMetadataPayload{Kind: "config", ConfigWatch: items}
	data, err := json.Marshal(payload)
	if err != nil {
		r.logCtx.GetBaseLogger().Warnf("marshal config metadata failed: %v", err)
		return
	}
	req.ConfigMetadata = string(data)
}

// TriggerNow 请求触发一次即时 ReportClient 上报，不等定时周期。
// 供配置文件监听列表变化时调用，使服务端尽快感知新的监听配置。
//
// 采用 debounce 合并：短时间内的多次调用只会产生一次实际上报——
// 应用启动时常批量订阅数十个配置文件，若每次都直接上报会瞬间打出等量并发 gRPC 请求，
// 造成单客户端上报风暴（服务端侧可能触发限流，客户端侧连接池竞争）。
// 首次调用会等待 reportDebounceWindow 让后续订阅合并进来，窗口结束后统一上报一次。
// 上报失败仅记日志，不返回错误，避免影响触发方（如配置订阅流程）。
func (r *ReportClientCallBack) TriggerNow() {
	// 已有待处理的合并请求时直接返回，由那次上报覆盖本次变化
	if !atomic.CompareAndSwapInt32(&r.reportPending, 0, 1) {
		return
	}
	go func() {
		defer atomic.StoreInt32(&r.reportPending, 0)
		// 合并窗口：等待期间到来的 TriggerNow 都被本次上报覆盖
		time.Sleep(reportDebounceWindow)
		r.doReportNow()
	}()
}

// doReportNow 立即执行一次 ReportClient 上报。
// 与定时任务 Process 可能并发调用 connector.ReportClient，依赖 connector 的线程安全
// （其实现每次独立获取连接）。
func (r *ReportClientCallBack) doReportNow() {
	reportClientReq := r.reportClientRequest()
	if err := reportClientReq.Validate(); err != nil {
		r.logCtx.GetBaseLogger().Errorf("trigger now report client validate error: %v", err)
		return
	}
	if _, err := r.connector.ReportClient(reportClientReq); err != nil {
		// 上报失败不影响业务，定时任务下一周期会重试，故仅 warn 不 error（避免触发告警噪音）
		r.logCtx.GetBaseLogger().Warnf("trigger now report client error: %v", err)
	}
}

// configMetadataPayload ReportClient 上报 config_metadata 字段的 JSON 结构
type configMetadataPayload struct {
	Kind        string                              `json:"kind"`
	ConfigWatch []configflow.ConfigFileMetadataItem `json:"config_watch"`
}
