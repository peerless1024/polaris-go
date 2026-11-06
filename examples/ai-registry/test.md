# AI Registry 真实 E2E

本文档与 `verify_e2e.sh`、`cleanup.sh` 及其调用的 `scenarios.go` 保持一致。只有实际真实服务端运行成功才算 E2E 通过；本地 httptest、mock 套件、race 不替代真实验收。

## 前置条件

- Go 和 Bash；依赖与根模块一致，不新增 module。清理用 `ps` 验证进程完整命令。
- 真实 Polaris 支持 `AI_AGENT` 服务投影、服务 extended metadata 替换和显式空列表、Skill Registry 多文件发布/下载、配置分组订阅。
- 端口默认 Discover `8091`、Config `8093`、Skill `8094`、Console `8090`，可通过 `--discover`、`--config-address`、`--skill`、`--console` 显式覆盖。
- `--namespace` 默认 `go-sdk-integration`，拒绝 `default`；默认 run ID 为
  `yyyyMMddHHmmssSSS`，服务名为 `go-sdk-e2e-<run-id>-<label>`，owner 为
  `polaris-go-ai-registry`，服务/实例附带 `e2e.run.id`。
- 可复用 fixture：`polaris-go-ai-e2e-a-weather`（`1.0.0`、`2.0.0`），`polaris-go-ai-e2e-z-currency`（`1.0.0`），`polaris-go-ai-e2e-draft`（草稿）。同名未归属 fixture 会被拒绝。Weather ZIP 有两个 640 KiB 不可压缩附件，总量约 1.25 MiB；每个文件低于服务端 1 MiB 上限。
- Console token fallback：`POLARIS_CONSOLE_TOKEN` → `POLARIS_TOKEN` → `POLARIS_E2E_SERVICE_TOKEN` → `POLARIS_SERVICE_TOKEN`；Skill 为 `POLARIS_SKILL_TOKEN` → `POLARIS_TOKEN`，Config 为 `POLARIS_CONFIG_TOKEN` → `POLARIS_TOKEN`，Provider 为 `POLARIS_E2E_SERVICE_TOKEN` → `POLARIS_SERVICE_TOKEN`。无默认 token。`--config-file` 可加载本地配置，环境变量优先；状态和报告不含 token。

## 运行流程

```bash
bash examples/ai-registry/verify_e2e.sh --server <polaris-host>
bash examples/ai-registry/agent/verify.sh --server <polaris-host>
bash examples/ai-registry/mcp/verify.sh --server <polaris-host>
bash examples/ai-registry/skill/verify.sh --server <polaris-host> --timeout 60s
bash examples/ai-registry/verify_e2e.sh --server <polaris-host> --services --processes
bash examples/ai-registry/cleanup.sh --state .logs/<run-id>/state.json --services --processes --dry-run
bash examples/ai-registry/cleanup.sh --state .logs/<run-id>/state.json --services --processes --apply
```

根入口默认执行 `all` suite；子目录入口只执行对应 suite。suite、namespace、run ID 和真实 E2E
结果均写入 `result.json`，清理仍以 state 中的 owner/run 为准。
也可在根入口显式使用 `--suite agent|mcp|skill`；`POLARIS_E2E_SUITE` 可提供相同选择。

`--timeout` 默认 10s，完整 `--run-timeout` 默认 12m，须大于单次超时。每个 eventual 场景以 200ms 间隔在其 deadline 内重试，失败仍记录为 FAIL，不静默 skip。输出 `.logs/<run-id>/state.json`、`result.json` 和独立 Provider 日志；清理之后才写最终 success。

低带宽远端可显式用 `--timeout 2m --run-timeout 35m`。该参数会同时设置该次 E2E 的 Skill 消息与 ZIP 下载预算，不修改 SDK 或 `polaris.yaml` 的默认值，也不减少测试数据；Provider ready 预算为 timeout + 35s。多帧 ZIP 与内容检查保持不变。

默认保留进程和服务。选择 `--services` 时同时选择 `--processes`，先停止 Provider 防止重新注册，再按 owner/run 验证并删除服务及实例；配置组验证 comment 的 owner/run 后删除。Skill 默认保留。`--apply`、`--force` 或 `-f` 才执行 cleanup 删除；`--dry-run` 优先，保留并展示资源选择，禁止写入和停止。`--dry-run` 仅允许 cleanup，verify 提前拒绝此参数。日志和结果保留供 CR 追溯。

## Agent 用例

### [用例 1.1 shared-sdk-context]
操作：创建四个公开 API。预期：全部共享同一 SDKContext。

### [用例 1.2 ensure-agent-service-and-idempotency]
操作：Ensure 空 Skill 的 AI_AGENT，再携带不同 metadata/Skill Ensure。预期：返回完整 revision/type/metadata，已有服务不被修改。

### [用例 1.3 ordinary-service-type-conflict]
操作：创建普通微服务，调用 Agent Ensure。预期：15s 内返回 ServiceTypeConflict，不更改类型。

### [用例 1.4 agentcard-only-auto-registration]
操作：独立 Provider 仅实例 metadata 注册，不预建 AI 服务。预期：自动创建 AI_AGENT、服务无挂载 Skill，真实发现成功。

### [用例 1.5 discovered-agentcard-real-request]
操作：GetInstances、解析发现的 AgentCard、向其 URL 发请求。预期：回复匹配 service 和请求内容，不使用硬编码发现结果。

### [用例 1.6 automatic-heartbeat-remains-healthy]
操作：TTL=1 自动注册后等 5s，再发现。预期：实例依然健康、非隔离。

### [用例 2.1 mount-sequential-and-sorted-readback]
操作：先挂 currency，再 weather，重复挂 weather。预期：两 ID 都保留，排序回读和幂等成功。

### [用例 2.2 update-skill-fields-and-preserve-service]
操作：设置 export/sync 属性，再更新同 ID Skill description。预期：完整字段更新，服务 metadata/export/sync 不丢失。

### [用例 2.3 discover-mounted-skills-and-clone-snapshot]
操作：从 Discover 读取两个挂载 Skill，修改返回对象再读取。预期：25s 内挂载可见，调用者修改不污染缓存。

### [用例 2.4 normal-services-and-agent-concurrent-query]
操作：相同 namespace/service 的普通 GetServices 与 Agent INSTANCE 读取各并发 12 次。预期：24 次都成功，响应无错配。

### [用例 2.5 replace-and-unmount-last-skill]
操作：整体替换为一个 Skill，卸载至空，再挂载。预期：每次返回完整目标，显式空数组真实生效。

### [用例 2.6 same-owner-concurrent-skill-writes]
操作：独立 Agent 服务预置 weather，通过同一个 AgentAPI 同时更新 weather 能力与追加 currency，连续三轮。预期：两个写入均确认成功后回读包含两 ID 与本轮更新的完整 description，不能因同 owner 并发覆盖前写；服务随本轮显式清理。

## Skill 用例

### [用例 3.1 published-explicit-and-active-version]
操作：GetSkill 查询 weather 1.0.0、2.0.0 和省略版本。预期：省略版本返回 active 1.0.0；YAML front matter 的 name/description/examples 与发布的资源元数据一致，正文逐字节匹配。服务端合法重写 front matter 不被误判为内容篡改。

### [用例 3.2 draft-and-missing-skill-errors]
操作：查询草稿和不存在 Skill。预期：都返回 SkillNotFound。

### [用例 3.3 published-filter-and-pagination]
操作：在隔离 namespace 中使用 limit=1、offset=0/1/2 分页，再按 name 过滤，并直接校验返回资源的 fixture tag。
预期：两个 published 资源各一次，无草稿泄露，最后页空，两个资源都带 fixture tag。

### [用例 3.4 markdown-download-and-shared-input]
操作：8 goroutine 共享省略 format 的 request 下载 Markdown。预期：front matter 元数据和正文校验、version/默认 format 正确，原 request 不被修改。

### [用例 3.5 multiframe-zip-and-file-integrity]
操作：真实下载大于 1 MiB ZIP，解包逐文件比对。预期：多帧重组完整，SKILL.md front matter 与资源元数据一致、正文和附件逐字节相同，无丢失或额外文件。

### [用例 4.1 install-and-cache-hit]
操作：同 pinned 版本安装两次。预期：首次 miss，第二次验证过的 cache hit，Dir/Digest 相同。

### [用例 4.2 tampered-entry-and-nested-file-recovery]
操作：依次篡改 SKILL.md 和嵌套 `.polaris-skill.json` 再安装。预期：均不是 cache hit，重新安装所有正确文件。

### [用例 4.3 forced-and-concurrent-install]
操作：Force 重装，再并发 12 次安装。预期：Force 不是 hit，所有安装结果完整。

### [用例 4.4 active-version-change-and-pinned-cache]
操作：管理端切 active 为 2.0.0，省略版本 Install，再 pinned 1.0.0。预期：25s 内新 active 内容可见，旧 pinned 缓存仍可用且路径不同。

### [用例 4.5 provider-loads-published-skill-and-invokes]
操作：独立 Provider pinned Install 1.0.0，以安装 Dir 拼接包内相对 EntryFile 读取指令，发现后发业务请求。预期：回复含实际安装的 1.0.0 指令。

### [用例 5.3 provider-deregister-and-stop-heartbeat]
操作：SIGTERM Skill Provider，等发现移除，再等 3s。预期：25s 内实例消失，停止心跳后不重新注册。

## Agent 公共回归

### [用例 5.1 healthy-and-isolated-instance-filtering]
操作：同服务注册隔离实例，再查 GetAllInstances 和 GetInstances。预期：all 能见隔离实例，filtered 不能见；随后反注册。

### [用例 5.2 configuration-read-watch-and-cancel]
操作：独立配置组发布 generation 0 并记录版本、订阅、发布并获取 generation 1、取消两次、发布 generation 2 并获取。预期：第一轮内容正确且版本递增，callback 的 namespace/group/name/version 与 generation 1 发布完全匹配，提供 MD5 时必须相等，旧版本或无关事件不能通过；分组列表省略 MD5 时，以相同文件/版本关联已实际读取并校验的发布内容。取消后第二轮版本递增、内容可读且不再回调。

### [用例 5.4 sdk-destroy-rejects-agent-and-skill]
操作：独立 Context 先真实查询 Skill，再 Destroy，调用 Agent/Skill API。预期：都返回 InvalidStateError。

## MCP 用例

### [用例 6.1 mcp-streamable-http-metadata-and-discovery]
操作：注册 `mcp-streamable-http` 普通实例，写入 `mcp-server-name`、`mcp-protocol-version`、
`mcp-endpoint-path`。预期：protocol、version、metadata、发现和 endpoint 均一致。

### [用例 6.2 mcp-sse-endpoint-and-metadata]
操作：注册 `mcp-sse` 实例。预期：SSE metadata、protocol 和 `/sse` endpoint 正确。

### [用例 6.3 mcp-heartbeat-and-deregister]
操作：TTL 1 秒注册，等待超过 TTL 后复查健康状态，再反注册。预期：心跳保持实例健康，
反注册后实例在 deadline 内消失。

当前 MCP suite 不调用 Service Contract 或 `tools/list`；这些能力未在 `polaris-go` 公开实现时
不得作为通过标准。

## 失败排查

- 管理接口鉴权失败：使用上述 token 变量或本地配置；不在命令行传 token，不打印响应中的 access token。
- `Agent service response ... has no service-level revision` 或 `is not AI_AGENT`：检查 Discover INSTANCE 是否返回服务 revision，以及实例是否有 `ai-agent-name`、服务是否有 Skill extended metadata；不能换 mock 报 PASS。
- `ServiceWriteUnsupported` 或空列表确认超时：确认服务端 HTTP protobuf 解码保留 `extended_metadata: []` 的 nil/empty presence。legacy `jsonpb` 会把显式空数组解析成 nil，此时必须修复服务端解析，不能由客户端绕过。
- Skill fixture 不归属/内容不同：使用另一个隔离 namespace，不覆盖已有资源。
- 清理拒绝 unowned service/instance/group/PID：检查 state 和远端 owner/run，不能用 force 绕过。
- 本地 httptest 监听失败：属于工具沙箱限制，需在允许 localhost 网络的环境执行；与真实远端用例结果分开记录。
