# 配置生效查询云上验证

云上部署与验证物料，把 config-effect-demo 打包为单个 zip，上传到云节点直接运行二进制并验证配置生效查询。

## 目录结构

```
cloud/
├── build-materials.sh    生成部署物料(交叉编译 + 组装 + 打 zip)
├── cloud-clean.sh        清理物料产物(dist/、zip、临时二进制)
├── README.md             本文件
└── templates/            物料模板
    ├── polaris.yaml      配置模板(${POLARIS_SERVER}/${POLARIS_TOKEN} 占位)
    ├── client.sh         节点启动脚本(setup/start/stop/status/restart)
    ├── verify-cloud.sh   配置生效查询验证脚本(调服务端 maintain 接口)
    └── clean.sh          节点清理脚本
```

## 生成物料

```bash
cd cloud
./build-materials.sh            # 生成 dist/client/ + dist/client.zip
./build-materials.sh --clean    # 清理
```

生成 `dist/client.zip`(约 13MB)，内含:

| 文件 | 说明 |
|------|------|
| `x86-bin` | config-effect-demo 预编译 Linux x86_64 静态二进制 |
| `polaris.yaml` | 配置模板(`POLARIS_SERVER`/`POLARIS_TOKEN` 环境变量占位) |
| `client.sh` | 启动/停止/状态/基线发布脚本 |
| `verify-cloud.sh` | 配置生效查询验证脚本 |
| `clean.sh` | 节点清理脚本 |

## 云上部署与验证

上传 `client.zip` 到云节点后:

```bash
unzip client.zip && cd client

# 1. 发布 3 份基线配置(base name 派生 -1/-2/-3.yaml，已存在则跳过)，
#    并把第 1 份(-1.yaml)经 console 接口覆盖为加密配置(encrypt_algo=AES)后发布
POLARIS_TOKEN=xxx ./client.sh setup --polaris-server <服务端地址> --content effect-content-v

# 2. 启动常驻客户端(自动订阅配置 + 建 WatchClientEvents 长连接 + 暴露 HTTP)
POLARIS_TOKEN=xxx ./client.sh start --polaris-server <服务端地址> --port 18091

# 3. 查看状态(显示 clientID 与本地生效配置)
./client.sh status --port 18091

# 4. 执行配置生效查询验证(调服务端 maintain 接口，校验 ACK)
POLARIS_TOKEN=xxx ./verify-cloud.sh --polaris-server <服务端地址> \
    --maintain-port 8090 --client-port 18091

# 5. 停止与清理
./client.sh stop
./clean.sh -f
```

## 加密配置（第 1 个文件）

第 1 个派生文件（`-1.yaml`）作为**加密配置**参与验证，覆盖「密文下发 → SDK crypto filter 解密 → 明文生效 → ACK 携带加密元信息 → 接收方解密」的完整链路。

- **创建方式**：SDK 的 `CreateConfigFile` 不带 `Encrypted`/`Tags`，无法创建加密配置。`client.sh setup` 在 SDK 明文基线之后，改用服务端 console HTTP 接口（`POST /config/v1/configfiles`，与 maintain 同端口，body 带 `encrypted:true, encrypt_algo:"AES"`）覆盖创建并发布；重复执行具有自纠正性。
- **客户端解密**：crypto/aes filter 默认启用（非 agent 模式），客户端订阅到加密的 `-1.yaml` 会自动解密，`/config` 快照的 `content` 为生效明文。
- **ACK 加密元信息**：加密配置的 ACK 回带 `content` 为**源内容（密文）**，并额外携带 `encrypted:true`、`encrypt_algo`、`data_key`（base64 明文数据密钥），接收方可据此解密（`AES-CBC-PKCS7`，IV 取 `key[:16]`，与 SDK `plugin/configfilter/crypto/aes` 实现对齐）。`verify-cloud.sh` 校验 5.1 断言元信息齐全，校验 5.2 用 `data_key` 经 openssl 解密 ACK 密文并断言等于客户端 `/config` 的生效明文。

## 验证原理

```
                ┌───────────────┐   ReportClient(clientID)    ┌──────────────┐
                │   client      │ ──────────────────────────► │              │
                │ (config-effect│   WatchClientEvents(WATCH)  │              │
                │   -demo)      │ ◄──────────────────────────► │  Polaris     │
                │               │   PUSH ──────────────────►  │  服务端      │
                │ 订阅配置文件   │   ◄──────────────── ACK     │  (商业版)    │
                └────┬──────────┘                             └──────┬───────┘
                     │ /clientid /config                             │
                     ▼                                               ▼
                ┌──────────────────────────────────────────────────────────┐
                │              verify-cloud.sh                            │
                │  1. 读客户端 /clientid 与 /config (version/md5/content) │
                │  2. 调服务端 maintain 接口 PUSH 配置生效查询              │
                │  3. 解析返回的 ACK content                              │
                │  4. 断言 applied=true 且 version/md5/content 一致        │
                │  5. 加密文件 ACK 携带 encrypt_algo/data_key，            │
                │     用 data_key 解密密文后断言 == 客户端生效明文          │
                └──────────────────────────────────────────────────────────┘
```

## 前置条件

1. 北极星服务端(商业版)已启动，且已实现 `WatchClientEvents` 接口
2. 服务端 maintain HTTP 端口可达(默认 `8090`，可用 `--maintain-port` 指定)
3. 云节点有执行权限与 `curl`/`python3`/`openssl`(`verify-cloud.sh` 解析 JSON 与解密加密 ACK 依赖)
4. 配置文件组 `polaris-config-example` 已存在或客户端有创建权限
5. 服务端 console 配置接口(`/config/v1/configfiles`，与 maintain 同端口)可达，且 `POLARIS_TOKEN` 具备配置写权限(`client.sh setup` 创建/发布加密文件依赖)

## 故障排查

| 现象 | 可能原因 |
|------|----------|
| `client.sh start` 进程立即退出 | polaris.yaml 地址/token 错误；查 `client.log` |
| `client.sh setup` 报"加密配置文件准备失败" | console 配置接口(`/config/v1/configfiles`，与 maintain 同端口)不可达，或 token 无配置写权限 |
| `verify-cloud.sh` 报"无 clientEvent.content" | WatchClientEvents 长连接未建立，查 `client.log` 是否有 `stream established` |
| `applied=false` | 客户端未订阅该配置文件(检查 `client.sh status` 的 version/md5 非空) |
| version/md5 不一致 | 客户端在 PUSH 时配置刚变更，重跑 `verify-cloud.sh` |
| 加密文件校验 5.1/5.2 失败 | 未先执行 `client.sh setup`(加密文件未就绪)、SDK crypto/aes filter 未启用(确认非 agent 模式)，或服务端下发的 `encrypt_algo` 不是 `AES` |
| maintain 接口鉴权失败 | `--polaris-token` 未传或无效 |
| `client_info.json` 残留 | 升级后持久化文件按 clientID 分文件，旧文件需 `clean.sh -f` 清理 |
