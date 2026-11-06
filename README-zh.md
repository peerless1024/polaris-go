# Polaris Go

[![codecov](https://codecov.io/gh/polarismesh/polaris-go/branch/main/graph/badge.svg)](https://app.codecov.io/gh/polarismesh/polaris-go)
[![Go](https://github.com/polarismesh/polaris-go/actions/workflows/testing.yml/badge.svg?branch=main)](https://github.com/polarismesh/polaris-go/actions)
[![goproxy](https://goproxy.cn/stats/github.com/polarismesh/polaris-go/badges/download-count.svg)](https://goproxy.cn/stats/github.com/polarismesh/polaris-go/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/polarismesh/polaris-go.svg)](https://pkg.go.dev/github.com/polarismesh/polaris-go)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/polarismesh/polaris-go?style=flat-square)](https://github.com/polarismesh/polaris-go)

[English](./README.md) | 简体中文

---

README：

- [介绍](#介绍)
- [如何使用](#如何使用)
- [使用示例](#使用示例)

## 介绍

polaris-go是北极星的Go语言嵌入式服务治理SDK。北极星是一个支持多种开发语言、兼容主流开发框架的服务治理中心。

polaris-go提供以下功能特性：

- 服务实例注册，心跳上报

  提供API接口供应用上下线时注册/反注册自身实例信息，并且可通过定时上报心跳来通知主调方自身健康状态。

- 服务发现

  提供多种API接口，通过API接口，用户可以获取服务下的全量服务实例，或者获取通过服务治理规则过滤后的一个服务实例，可供业务获取实例后马上发起调用。

- 故障熔断

  提供API接口供应用上报接口调用结果数据，并根据汇总数据快速对故障实例/分组进行隔离，以及在合适的时机进行探测恢复。

- 服务限流

  提供API接口供应用进行配额的检查及划扣，支持按服务级，以及接口级的限流策略。

- AI Agent 注册与 Skill 安装

  提供 `AgentAPI` 读取 AI Agent 服务和校验实例 metadata，提供 `SkillAPI` 查询、流式下载、安全解压和缓存
  published Skill。

## 如何使用

polaris-go通过go mod进行管理，用户可以在go.mod文件中引入polaris-go的依赖

```go
go get -u github.com/polarismesh/polaris-go
```

API的快速使用指南，可以参考：[QuickStart](examples/quickstart)

### AI Agent 与 Skill

Skill Registry 使用独立 gRPC 地址。Console 写入是可选能力，仅 `EnsureService`、`MountSkills` 和
`UnmountSkills` 需要配置。

```yaml
skill:
  enable: true
  addresses:
    - 127.0.0.1:8094
  connectTimeout: 1s
  messageTimeout: 5s
  downloadTimeout: 30s

serviceWriter:
  addresses:
    - http://127.0.0.1:8090
  timeout: 5s
```

```go
cfg := config.NewDefaultConfiguration([]string{"127.0.0.1:8091"})
cfg.GetSkill().SetEnable(true)
cfg.GetSkill().SetAddresses([]string{"127.0.0.1:8094"})

ctx, err := api.InitContextByConfig(cfg)
if err != nil {
	panic(err)
}
defer ctx.Destroy()

agentAPI := api.NewAgentAPIByContext(ctx)
skillAPI := api.NewSkillAPIByContext(ctx)

_, err = agentAPI.GetService(&api.GetAgentServiceRequest{
	GetAgentServiceRequest: model.GetAgentServiceRequest{
		Namespace: "Production",
		Service:   "customer-agent",
	},
})
if err != nil {
	panic(err)
}
_, err = skillAPI.GetSkill(&api.GetSkillRequest{
	GetSkillRequest: model.GetSkillRequest{
		Namespace: "Production",
		Name:      "demo-skill",
		Version:   "1.0.0",
	},
})
if err != nil {
	panic(err)
}
_, err = skillAPI.Install(&api.SkillInstallRequest{
	SkillInstallRequest: model.SkillInstallRequest{
		Namespace: "Production",
		Name:      "demo-skill",
		Version:   "1.0.0",
		CacheDir:  "./var/polaris-skills",
	},
})
```

Provider、Consumer、Agent 和 Skill API 应复用同一个 `SDKContext`。未配置 `serviceWriter.addresses` 时，
服务级 Skill 写入返回 `ErrCodeServiceWriteUnsupported`；通过实例注册自动创建 AI Agent 服务仍然可用。

同一 `SDKContext` 对同一个服务的 `EnsureService`、`MountSkills`、`UnmountSkills` 串行执行；
锁等待、HTTP 读改写和回读确认共享 `serviceWriter.timeout`。跨 `SDKContext` 或进程的写入仍需
调用方协调单 owner，不保证多客户端并发合并。

`AgentAPI.GetService` 使用 Discover INSTANCE 响应携带的服务投影，与 `polaris-java` 的
`getAllInstances` 读取路径一致。显式 `service_type` 会原样保留；为兼容 Java 注册及仍把类型投影为默认
`MICROSERVICE` 的服务端，实例 `ai-agent-name` 或 Skill extended metadata 会被识别为 AI Agent。该投影
必须包含 `revision`。清空最后一个 Skill 依赖服务端在 HTTP protobuf 解码中保留 repeated 字段 presence；
未支持时 SDK 返回 `ErrCodeServiceWriteUnsupported`。

AgentCard 的最小契约：

- metadata key 为 `ai-agent-name`、`a2a.agent.card`、`a2a.transport`、`a2a.path`；
- `a2a.agent.card` 必须是合法 JSON，card 的 `name` 与 `ai-agent-name` 一致；
- `a2a.path` 与首选 `supportedInterfaces[0].url` 的 path 一致；
- `a2a.transport` 与首选 interface 的 `protocolBinding` 一致。

Skill 安装会拒绝 ZIP Slip、绝对路径、符号链接、超过 64 MiB 的单文件和超过 256 MiB 的包；ZIP
最多包含 10000 个 entry，实际解压字节数同样受 256 MiB 限制。完整校验后发布不可变缓存代目录，
此前返回的安装路径在 Force 或修复安装期间仍可读取。缓存 tree digest 覆盖文件内容和权限位。
最后一次成功 Force 的选择会原子持久化，当前代目录损坏时重新下载修复，不会静默回退到旧缓存包。
当前验证范围见 `CHANGELOG.md`。

## 使用示例

为了演示功能如何使用，polaris-go 项目包含了一个子模块polaris-examples。此模块中提供了演示用的 example ，您可以阅读对应的 example 工程下的 README-zh 文档，根据里面的步骤来体验。

- [快速开始](https://github.com/polarismesh/polaris-go/tree/main/examples/quickstart)
- [路由示例](https://github.com/polarismesh/polaris-go/tree/main/examples/route)
- [熔断示例](https://github.com/polarismesh/polaris-go/tree/main/examples/circuitbreaker)
- [限流示例](https://github.com/polarismesh/polaris-go/tree/main/examples/ratelimit)
- [配置中心示例](https://github.com/polarismesh/polaris-go/tree/main/examples/configuration)

