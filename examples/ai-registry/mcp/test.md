# MCP Suite

本目录验证普通 Go SDK 的 MCP Instance 注册、协议/metadata、发现、心跳和反注册。

当前 `polaris-go` 没有 MCPAPI、Service Contract 和 MCP Client，因此本 suite 不验证
`tools/list`，也不把 Service Contract 当作通过条件。完整 MCP 契约测试需要后续 API 和真实
MCP Server。
默认 namespace 为 `go-sdk-integration`。

## 运行

```bash
bash examples/ai-registry/mcp/verify.sh --server <polaris-host>
bash examples/ai-registry/mcp/cleanup.sh \
  --state examples/ai-registry/.logs/<run-id>/state.json --services --apply
```

## 用例

- `6.1` `mcp-streamable-http`：校验 protocol、服务版本、MCP metadata、发现和 URL；
- `6.2` `mcp-sse`：校验 SSE metadata、路径和隔离的协议实例；
- `6.3` `mcp-heartbeat-and-deregister`：TTL 1 秒，等待心跳后确认健康，再反注册并等待消失。

固定 metadata：

- `mcp-server-name`
- `mcp-protocol-version`
- `mcp-endpoint-path`

不依赖其他示例的固定服务名或 fixture；服务名由 Go run ID 生成。
