# AI Registry 真实集成示例

该示例属于根 Go module。一个共享 SDKContext 同时提供 AgentAPI、SkillAPI、ConsumerAPI、ProviderAPI 和配置 API；独立 Provider 进程注册 AgentCard、自动心跳，Consumer 从真实 Polaris 发现实例后发起 HTTP 请求。Skill Provider 下载并安装已发布的 Skill，把实际安装内容用于业务响应。

需要可访问的 Polaris Discover、Skill、Config gRPC 和 Console HTTP 服务。完整场景、前置条件和清理规则见 [test.md](test.md)。此处的 HTTP echo 验证注册、发现和已加载 Skill 的调用链，不实现 A2A 推理协议或调用外部 LLM。

```bash
bash examples/ai-registry/verify_e2e.sh --server <polaris-host>
```

默认使用独立 `go-sdk-integration` 命名空间，服务名携带唯一 run ID。用例按 Agent、MCP、Skill
拆分为三个子目录；默认入口运行全部 suite：

```bash
bash examples/ai-registry/verify_e2e.sh --server <polaris-host> --services --processes
bash examples/ai-registry/agent/verify.sh --server <polaris-host> --services --processes
bash examples/ai-registry/mcp/verify.sh --server <polaris-host>
bash examples/ai-registry/skill/verify.sh --server <polaris-host> --timeout 60s
```

脚本默认保留 Provider 进程、服务、Skill 和诊断日志；验证器的取消监听、反注册测试会停止其专用
Skill Provider，但保留 AgentCard Provider 供检查。显式自清理运行：

```bash
bash examples/ai-registry/cleanup.sh \
  --state examples/ai-registry/.logs/<run-id>/state.json --services --processes --apply
```

各 suite 的用例和边界见 [agent/test.md](agent/test.md)、[mcp/test.md](mcp/test.md) 和
[skill/test.md](skill/test.md)。
根 runner 也支持 `--suite all|agent|mcp|skill`，默认 `all`。

默认不会写入任何部署地址或 token。Token 只通过环境变量或本地 SDK 配置读取；Console 使用 `POLARIS_CONSOLE_TOKEN`、`POLARIS_TOKEN`、`POLARIS_E2E_SERVICE_TOKEN`、`POLARIS_SERVICE_TOKEN` 的第一个非空值。Skill 使用 `POLARIS_SKILL_TOKEN` 或 `POLARIS_TOKEN`，配置使用 `POLARIS_CONFIG_TOKEN` 或 `POLARIS_TOKEN`，注册使用 `POLARIS_E2E_SERVICE_TOKEN` 或 `POLARIS_SERVICE_TOKEN`。环境变量未提供时保留 `--config-file` 中相应配置，不内置凭据。

清理要求明确状态文件；默认仅展示，不停止进程、不删除服务：

```bash
bash examples/ai-registry/cleanup.sh --state .logs/<run-id>/state.json
bash examples/ai-registry/cleanup.sh --state .logs/<run-id>/state.json --services --processes --apply
```

默认保留可复用 Skill。仅在明确希望删除本示例持有的三个 fixture 时添加 `--skills`；所有删除都验证 owner/run，`--force` 也不绕过归属检查。MCP suite 当前只验证注册元数据、发现、心跳和反注册；MCP Service Contract/Tool 调用不在当前 Go SDK 范围内。
