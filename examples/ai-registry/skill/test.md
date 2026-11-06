# Skill Suite

本目录验证 published/active Skill 查询、Markdown/ZIP 下载、Go 扩展 Install、缓存修复、
Force 和 active 切换。
默认 namespace 为 `go-sdk-integration`。

## 运行

```bash
bash examples/ai-registry/skill/verify.sh --server <polaris-host> --timeout 60s
bash examples/ai-registry/skill/cleanup.sh \
  --state examples/ai-registry/.logs/<run-id>/state.json --services --apply
```

## 用例

- `3.1` 至 `3.5`：published/active/pinned 查询、错误语义、分页、Markdown、多帧 ZIP；
- `4.1` 至 `4.5`：缓存命中、篡改修复、Force/并发、active 切换和独立 Provider 加载；
- `5.3`：停止 Skill Provider 并确认反注册/心跳停止。

Go fixture 使用 `polaris-go-ai-e2e-*`；不要复用其他示例的固定 Skill 名称、
fixture 或默认 namespace。Skill 默认保留，只有显式 `--skills` 才删除。
