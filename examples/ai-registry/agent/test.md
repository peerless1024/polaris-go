# Agent Suite

本目录验证 AI Agent 服务、AgentCard、服务级 Skill 挂载和注册生命周期。默认 namespace 为
`go-sdk-integration`，不得改为 `default`。

## 运行

```bash
bash examples/ai-registry/agent/verify.sh --server <polaris-host> --services --processes
bash examples/ai-registry/agent/cleanup.sh \
  --state examples/ai-registry/.logs/<run-id>/state.json --services --processes --apply
```

## 用例

- `1.1` 至 `1.6`：共享 SDKContext、Ensure、类型冲突、AgentCard-only 注册、实际 HTTP 请求和心跳；
- `2.1` 至 `2.6`：挂载 Skill、字段更新、Agent 发现快照、并发读取和同 owner 并发写；
- `5.1`、`5.4`：实例过滤和 SDKContext 销毁；
- `5.2`：受影响配置中心订阅回归。

本 suite 会使用 `polaris-go-ai-e2e-*` 的 Agent/Skill fixture。Skill 默认保留，服务和进程
通过显式 `--services --processes` 清理。
