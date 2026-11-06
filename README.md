# Polaris Go

[![codecov](https://codecov.io/gh/polarismesh/polaris-go/branch/main/graph/badge.svg)](https://app.codecov.io/gh/polarismesh/polaris-go)
[![Go](https://github.com/polarismesh/polaris-go/actions/workflows/testing.yml/badge.svg?branch=main)](https://github.com/polarismesh/polaris-go/actions)
[![goproxy](https://goproxy.cn/stats/github.com/polarismesh/polaris-go/badges/download-count.svg)](https://goproxy.cn/stats/github.com/polarismesh/polaris-go/badges/download-count.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/polarismesh/polaris-go.svg)](https://pkg.go.dev/github.com/polarismesh/polaris-go)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/polarismesh/polaris-go?style=flat-square)](https://github.com/polarismesh/polaris-go)

English | [简体中文](./README-zh.md)

README：

- [Introduction](#introduction)
- [How to use](#how-to-use)
- [Examples](#examples)

## Introduction

Polaris-go is golang SDK for Polaris. Polaris is an operation centre that supports multiple programming languages, with high compatibility to different
application framework.

Polaris-go provide features listed as below:

- Service instance registration, and health check

  Provides API on/offline registration instance information, with regular report to inform caller server's healthy
  status.

- Service discovery

  Provides multiple API, for users to get a full list of server instance, or get one server instance after route rule
  filtering and loadbalancing, which can be applied to srevice invocation soon.

- Service circuitbreaking

  Provide API to report the invocation result, and conduct circuit breaker instance/group insolation based on collected
  data, eventually recover when the system allows.

- Service ratelimiting

  Provides API for applications to conduct quota check and deduction, supports rate limit policies that are based on
  server level and port.

- AI Agent registry and Skill installation

  Provides `AgentAPI` for AI Agent service discovery and instance metadata validation, and `SkillAPI` for querying,
  streaming download, safe extraction, and local caching of published Skills.

## How to use

polaris-go can be referenced by go mod, user can add dependency to go.mod file

```go
go get -u github.com/polarismesh/polaris-go
```

API quick start guide，can reference：[QuickStart](examples/quickstart)

### AI Agent and Skill

Skill Registry uses an independent gRPC endpoint. Console service writes are optional and are only needed for
`EnsureService`, `MountSkills`, and `UnmountSkills`.

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

The same `SDKContext` should be reused by Provider, Consumer, Agent, and Skill APIs. Without `serviceWriter.addresses`,
service-level Skill writes return `ErrCodeServiceWriteUnsupported`; instance-registration-based service creation
remains available.

Within one `SDKContext`, writes to the same service through `EnsureService`, `MountSkills`, and `UnmountSkills`
are serialized. Lock waiting, HTTP read-modify-write, and confirmation share `serviceWriter.timeout`. Writers
across contexts or processes still require a single coordinated owner; multi-client concurrent merging is not guaranteed.

`AgentAPI.GetService` uses the service projection carried by a Discover INSTANCE response, matching
`polaris-java`'s `getAllInstances` read path. Explicit `service_type` is preserved; for compatibility with
Java registrations and older projections that default the type to `MICROSERVICE`, a non-empty instance
`ai-agent-name` or a Skill extended-metadata entry identifies an AI Agent. The projection must contain
`revision`. Clearing the final Skill depends on server-side HTTP protobuf support for repeated-field presence;
when unavailable, the SDK returns `ErrCodeServiceWriteUnsupported`.

Skill installation rejects ZIP Slip, absolute paths, symlinks, individual files larger than 64 MiB, and packages
larger than 256 MiB. ZIP archives are limited to 10000 entries, and the actual extracted byte count is capped even if
archive headers are inconsistent. After full validation, content is published to an immutable cache generation, so
paths returned by earlier installations remain readable during forced or repair installations. The cache tree digest
covers file content and permission bits. The last successful Force selection is persisted atomically; a damaged current
generation is downloaded and repaired instead of silently falling back to an older cached package.

## Examples

A polaris-examples module is included in our project for you to get started with polaris-go quickly. It contains an example, and you can refer to the readme file in the example project for a quick walkthrough.

- [quick start](https://github.com/polarismesh/polaris-go/tree/main/examples/quickstart)
- [router example](https://github.com/polarismesh/polaris-go/tree/main/examples/route)
- [circuit breaker example](https://github.com/polarismesh/polaris-go/tree/main/examples/circuitbreaker)
- [rate limiter example](https://github.com/polarismesh/polaris-go/tree/main/examples/ratelimit)
- [config center example](https://github.com/polarismesh/polaris-go/tree/main/examples/configuration)

