package pb

import (
	"github.com/golang/protobuf/proto"
	"github.com/modern-go/reflect2"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
	apitraffic "github.com/polarismesh/specification/source/go/api/v1/traffic_manage"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/polarismesh/polaris-go/pkg/model"
)

// NearbyRoutingAssistant 就近路由规则解析助手
type NearbyRoutingAssistant struct {
}

// ParseRuleValue 解析出具体的规则值
func (n *NearbyRoutingAssistant) ParseRuleValue(resp *apiservice.DiscoverResponse) (proto.Message, string) {
	var revision string
	if resp.NearbyRouteRules == nil || len(resp.NearbyRouteRules) == 0 {
		// 当没有就近路由规则时，使用 service.revision
		revision = resp.GetService().GetRevision().GetValue()
		return nil, revision
	}

	// 如果有就近路由规则，从第一个规则中获取revision并构造Routing对象
	rule := resp.NearbyRouteRules[0]
	revision = rule.GetRevision()
	routing := &apitraffic.Routing{
		Namespace: resp.Service.Namespace,
		Service:   resp.Service.Name,
		Rules:     resp.NearbyRouteRules,
		Revision:  wrapperspb.String(revision),
	}
	return routing, revision
}

// Validate 规则校验
func (n *NearbyRoutingAssistant) Validate(message proto.Message, ruleCache model.RuleCache) error {
	if reflect2.IsNil(message) {
		return nil
	}
	// 就近路由规则不需要特殊校验，复用RoutingAssistant的校验逻辑
	routingValue := message.(*apitraffic.Routing)
	assistant := &RoutingAssistant{}
	return assistant.Validate(routingValue, ruleCache)
}

// SetDefault 设置默认值
func (n *NearbyRoutingAssistant) SetDefault(message proto.Message) {
	// do nothing
}
