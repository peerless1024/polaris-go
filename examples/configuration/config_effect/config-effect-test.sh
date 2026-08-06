#!/bin/bash
# =============================================================================
# 配置生效查询验证脚本
#
# 验证 polaris-go 客户端通过 WatchClientEvents 长连接响应服务端「配置生效查询」：
#   - 客户端订阅配置文件后，SDK 自动建立 WatchClientEvents 双向流并上报 clientID
#   - 服务端 maintain 接口 (GET /maintain/v1/clients/event) 向指定客户端 PUSH 配置生效查询
#   - 客户端回 ACK (含 version/md5/applied)，服务端原样透传给本脚本
#   - 脚本解析 ACK content，校验 version/md5 与客户端本地生效配置一致
#
# 使用方法:
#   chmod +x config-effect-test.sh
#   ./config-effect-test.sh [--polaris-server <地址>] [--polaris-token <令牌>]
#                           [--maintain-port <端口>] [--namespace <命名空间>]
#                           [--group <配置组>] [--file <文件名>]
#                           [--port <客户端HTTP端口>] [--debug]
#
# 前置条件:
#   1. 北极星服务端(Polaris Server 商业版)已启动，且已更新含 WatchClientEvents 逻辑的版本
#   2. Go 环境已安装
#   3. 服务端 maintain HTTP 端口可达 (默认 8080，可用 --maintain-port 指定)
#
# 验证原理:
#   - 客户端启动后通过 ReportClient 上报 clientID，并建立 WatchClientEvents 长连接
#   - 脚本读取客户端 /clientid 与 /config，获得 clientID 与本地生效配置 version/md5
#   - 脚本调服务端 maintain 接口向该 clientID PUSH 查询 {kind:config, config:{ns,group,file}}
#   - 服务端通过 stream 下发 PUSH，客户端回 ACK，服务端把 ACK.clientEvent.content 透传回脚本
#   - 脚本解析 ACK content，断言 applied=true 且 version/md5 与客户端 /config 一致
# =============================================================================

set -euo pipefail

# ======================== 默认配置 ========================
POLARIS_SERVER="${POLARIS_SERVER:-127.0.0.1}"
POLARIS_TOKEN="${POLARIS_TOKEN:-}"
MAINTAIN_PORT="${MAINTAIN_PORT:-8080}"
NAMESPACE="${NAMESPACE:-default}"
FILE_GROUP="${FILE_GROUP:-polaris-config-example}"
FILE_NAME="${FILE_NAME:-config-effect-example.yaml}"
CLIENT_PORT="${CLIENT_PORT:-18091}"
DEBUG_MODE="${DEBUG_MODE:-false}"

# 验证用的配置内容(具有辨识度，便于自动判定)
CONFIG_CONTENT="effect-content-v1"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ======================== 解析命令行参数 ========================
while [[ $# -gt 0 ]]; do
    case "$1" in
        --polaris-server) POLARIS_SERVER="$2"; shift 2 ;;
        --polaris-token)  POLARIS_TOKEN="$2";  shift 2 ;;
        --maintain-port)  MAINTAIN_PORT="$2";  shift 2 ;;
        --namespace)      NAMESPACE="$2";      shift 2 ;;
        --group)          FILE_GROUP="$2";     shift 2 ;;
        --file)           FILE_NAME="$2";      shift 2 ;;
        --port)           CLIENT_PORT="$2";    shift 2 ;;
        --debug)          DEBUG_MODE="true";   shift ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --polaris-server <地址>  北极星服务端地址 (默认: 127.0.0.1)"
            echo "  --polaris-token <令牌>   北极星鉴权令牌 (默认: 空)"
            echo "  --maintain-port <端口>   服务端 maintain HTTP 端口 (默认: 8080)"
            echo "  --namespace <命名空间>   命名空间 (默认: default)"
            echo "  --group <配置组>         配置文件组 (默认: polaris-config-example)"
            echo "  --file <文件名>          配置文件名 (默认: config-effect-example.yaml)"
            echo "  --port <端口>            客户端 HTTP 观察端口 (默认: 18091)"
            echo "  --debug                  启用 debug 日志 (默认: 关闭)"
            exit 0
            ;;
        *)
            echo -e "${RED}未知参数: $1${NC}"; exit 1 ;;
    esac
done

# ======================== 全局变量 ========================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/.build"
LOG_DIR="${SCRIPT_DIR}/.logs"
PID_FILE="${SCRIPT_DIR}/.config-effect-pids"
RESULT_FILE="${LOG_DIR}/config_effect_result.csv"
PID_CLIENT=""

# ======================== 清理函数 ========================
cleanup() {
    log_info "清理客户端进程..."
    if [[ -n "$PID_CLIENT" ]] && kill -0 "$PID_CLIENT" 2>/dev/null; then
        kill "$PID_CLIENT" 2>/dev/null || true
        wait "$PID_CLIENT" 2>/dev/null || true
    fi
    if [[ -f "$PID_FILE" ]]; then
        while IFS= read -r pid; do
            [[ -n "$pid" ]] && kill "$pid" 2>/dev/null || true
        done < "$PID_FILE"
        rm -f "$PID_FILE"
    fi
}
trap cleanup EXIT

# ======================== 工具函数 ========================

log_info()  { echo -e "${GREEN}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*"; }
log_step() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  $*${NC}"
    echo -e "${CYAN}========================================${NC}"
}

# wait_for_http 轮询 HTTP 端口直到返回响应，同时检查进程存活。
# 入参: url max_wait desc pid
wait_for_http() {
    local url="$1" max_wait="${2:-30}" desc="${3:-服务}" pid="${4:-}"
    local waited=0
    while [[ $waited -lt $max_wait ]]; do
        if [[ -n "$pid" ]] && ! kill -0 "$pid" 2>/dev/null; then
            log_error "${desc} 进程 (PID: ${pid}) 已退出"
            return 1
        fi
        if curl -s --connect-timeout 2 "$url" > /dev/null 2>&1; then
            log_info "${desc} 已就绪 ($url)"
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    log_error "${desc} 未就绪 ($url)，等待了 ${max_wait}s"
    return 1
}

# get_client_id 从客户端 /clientid 接口获取 clientID。
get_client_id() {
    curl -s --connect-timeout 3 "http://127.0.0.1:${CLIENT_PORT}/clientid" 2>/dev/null
}

# get_config_field 从客户端 /config 接口提取指定字段。
# 入参: field (version|md5|content)
get_config_field() {
    local field="$1"
    curl -s --connect-timeout 3 "http://127.0.0.1:${CLIENT_PORT}/config" 2>/dev/null \
        | grep -oE "\"${field}\":[^,}]*" | head -1 | sed "s/\"${field}\"://;s/\"//g"
}

# ======================== 生成临时 polaris.yaml ========================
generate_polaris_yaml() {
    local target_file="$1"
    cat > "$target_file" <<EOF
global:
  serverConnector:
    addresses:
      - ${POLARIS_SERVER}:8091
    token: ${POLARIS_TOKEN}
    connectTimeout: 5000ms
  api:
    timeout: 5s
    maxRetryTimes: 2
    retryInterval: 1s
  eventReporter:
    enable: true
    chain:
      - pushgateway
config:
  configConnector:
    addresses:
      - ${POLARIS_SERVER}:8093
    token: ${POLARIS_TOKEN}
EOF
    log_info "生成 polaris.yaml -> ${target_file}"
}

# ======================== 进程管理 ========================
start_client() {
    local workdir="${BUILD_DIR}/client-run"
    local log_file="${LOG_DIR}/client.log"
    mkdir -p "$workdir"
    cp "${BUILD_DIR}/polaris-client.yaml" "${workdir}/polaris.yaml"

    local debug_flag=""
    [[ "$DEBUG_MODE" == "true" ]] && debug_flag="-debug"

    (cd "$workdir" && exec "${BUILD_DIR}/config-effect-demo" \
        -action=run \
        -config="${workdir}/polaris.yaml" \
        -namespace="$NAMESPACE" \
        -group="$FILE_GROUP" \
        -file="$FILE_NAME" \
        -port=":${CLIENT_PORT}" \
        $debug_flag \
        > "$log_file" 2>&1) &
    PID_CLIENT=$!
    echo "$PID_CLIENT" >> "$PID_FILE"
    log_info "客户端已启动 (PID: ${PID_CLIENT}, 端口: ${CLIENT_PORT}, 日志: ${log_file})"
}

# ======================== 核心验证：调服务端 maintain 接口下发 PUSH ========================
# query_config_effect 向服务端 maintain 接口请求对指定 clientID 下发配置生效查询。
# 返回值：服务端响应 JSON (apiservice.Response)，其中 clientEvent.content 为客户端 ACK content。
# 入参: client_id
query_config_effect() {
    local client_id="$1"
    # PUSH content：单点查询目标配置文件（kind=config + 三元组，snake_case）
    local push_content="{\"kind\":\"config\",\"config\":{\"namespace\":\"${NAMESPACE}\",\"group\":\"${FILE_GROUP}\",\"file_name\":\"${FILE_NAME}\"}}"
    local url="http://${POLARIS_SERVER}:${MAINTAIN_PORT}/maintain/v1/clients/event?client_id=${client_id}&content=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$push_content" 2>/dev/null || echo "$push_content")"
    log_info "调服务端 maintain: ${url}"
    local resp
    resp=$(curl -s --connect-timeout 10 --max-time 20 \
        -H "X-Polaris-Token: ${POLARIS_TOKEN}" \
        "$url" 2>/dev/null) || true
    echo "$resp"
}

# extract_ack_field 从服务端响应中提取 clientEvent.content 内的指定字段。
# 服务端响应结构：{ code, info, clientEvent: { client_id, index, content } }
# content 是 JSON 字符串，内含 { kind, config, version, md5, applied }
# 入参: resp_json ack_field (applied|version|md5)
extract_ack_field() {
    local resp="$1" field="$2"
    # 先取 clientEvent.content 字符串值，再在其中提取目标字段
    local content
    content=$(echo "$resp" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    ce = data.get('clientEvent') or {}
    print(ce.get('content', ''))
except Exception as e:
    sys.stderr.write(str(e) + '\n')
    sys.exit(1)
" 2>/dev/null) || {
        log_error "解析服务端响应失败，原始响应: $resp"
        return 1
    }
    if [[ -z "$content" ]]; then
        log_error "服务端响应无 clientEvent.content，原始响应: $resp"
        return 1
    fi
    log_info "客户端 ACK content: $content"
    echo "$content" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    val = data.get('${field}', '')
    print(val)
except Exception as e:
    sys.stderr.write(str(e) + '\n')
    sys.exit(1)
" 2>/dev/null
}

# record_result 记录一条用例结果到 CSV。
# 入参: case_id name status detail
record_result() {
    echo "$(date '+%Y-%m-%d %H:%M:%S'),$1,$2,$3,$4" >> "$RESULT_FILE"
}

# ======================== 主流程 ========================
main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          配置生效查询验证脚本                     ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "配置信息:"
    echo "  北极星服务端:       ${POLARIS_SERVER}"
    echo "  maintain 端口:      ${MAINTAIN_PORT}"
    echo "  命名空间/配置组:    ${NAMESPACE}/${FILE_GROUP}"
    echo "  配置文件名:         ${FILE_NAME}"
    echo "  客户端 HTTP 端口:   ${CLIENT_PORT}"
    echo "  Debug 日志:         ${DEBUG_MODE}"
    echo ""

    log_step "步骤 1/5 环境准备与编译"
    mkdir -p "$BUILD_DIR" "$LOG_DIR"
    echo "时间戳,用例编号,用例名称,结果,详情" > "$RESULT_FILE"
    : > "$PID_FILE"

    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    if ! command -v python3 &> /dev/null; then
        log_error "python3 未安装，脚本依赖 python3 解析 JSON"
        exit 1
    fi
    log_info "Go 版本: $(go version)"

    if [[ ! -f "${SCRIPT_DIR}/main.go" ]]; then
        log_error "找不到源码: ${SCRIPT_DIR}/main.go"
        exit 1
    fi

    log_info "编译 config-effect-demo..."
    (cd "$SCRIPT_DIR" && go build -o "${BUILD_DIR}/config-effect-demo" .)
    if command -v xattr &> /dev/null; then
        xattr -c "${BUILD_DIR}/config-effect-demo" 2>/dev/null || true
    fi
    log_info "编译完成 -> ${BUILD_DIR}/config-effect-demo"

    log_step "步骤 2/5 生成配置与发布全量基线"
    generate_polaris_yaml "${BUILD_DIR}/polaris-client.yaml"

    log_info "通过 config-effect-demo setup 准备全量基线(content=${CONFIG_CONTENT}, 已存在则跳过)..."
    (cd "${BUILD_DIR}" && "./config-effect-demo" \
        -action=setup \
        -config="${BUILD_DIR}/polaris-client.yaml" \
        -namespace="$NAMESPACE" -group="$FILE_GROUP" -file="$FILE_NAME" \
        -content="$CONFIG_CONTENT") || {
        log_error "全量基线准备失败，请确认配置组 ${FILE_GROUP} 存在且服务端可用"
        exit 1
    }

    log_step "步骤 3/5 启动客户端"
    start_client
    sleep 1
    kill -0 "$PID_CLIENT" 2>/dev/null || { log_error "客户端启动失败"; cat "${LOG_DIR}/client.log" 2>/dev/null; exit 1; }
    wait_for_http "http://127.0.0.1:${CLIENT_PORT}/health" 30 "客户端" "$PID_CLIENT" || exit 1

    log_step "步骤 4/5 验证客户端已拉取基线配置"
    local client_version client_md5 client_content
    # 轮询 /config 直到拿到非空 version/md5
    local waited=0
    while [[ $waited -lt 30 ]]; do
        client_version=$(get_config_field "version")
        client_md5=$(get_config_field "md5")
        client_content=$(get_config_field "content")
        if [[ -n "$client_version" && "$client_version" != "0" && -n "$client_md5" ]]; then
            break
        fi
        sleep 1
        waited=$((waited + 1))
    done
    if [[ -z "$client_version" || "$client_version" == "0" || -z "$client_md5" ]]; then
        log_error "客户端未拉取到配置文件 (version=${client_version}, md5=${client_md5})"
        record_result "0" "客户端拉取配置" "FAIL" "version=${client_version},md5=${client_md5}"
        exit 1
    fi
    log_info "客户端本地生效配置: version=${client_version}, md5=${client_md5}, content=${client_content}"
    record_result "0" "客户端拉取配置" "PASS" "version=${client_version},md5=${client_md5}"

    log_step "步骤 5/5 通过服务端 maintain 接口下发配置生效查询并校验 ACK"
    local client_id
    client_id=$(get_client_id)
    if [[ -z "$client_id" ]]; then
        log_error "获取 clientID 失败"
        record_result "1" "获取 clientID" "FAIL" "客户端 /clientid 为空"
        exit 1
    fi
    log_info "客户端 clientID: ${client_id}"
    record_result "1" "获取 clientID" "PASS" "clientID=${client_id}"

    # 等待 WatchClientEvents 长连接建立（SDK Start 后异步建立，留足时间）
    log_info "等待 WatchClientEvents 长连接建立 (5s)..."
    sleep 5

    local resp
    resp=$(query_config_effect "$client_id")
    log_info "服务端响应: ${resp}"

    local ack_applied ack_version ack_md5
    ack_applied=$(extract_ack_field "$resp" "applied") || { record_result "2" "解析 ACK" "FAIL" "解析失败"; exit 1; }
    ack_version=$(extract_ack_field "$resp" "version") || true
    ack_md5=$(extract_ack_field "$resp" "md5") || true

    local overall_pass=true

    # 校验 1：applied 必须为 true（客户端确实在监听该配置文件）
    if [[ "$ack_applied" == "True" ]]; then
        log_info "✅ [用例 2.1 ACK applied=true] PASS - 客户端确认监听该配置文件"
        record_result "2.1" "ACK applied=true" "PASS" "applied=True"
    else
        log_error "❌ [用例 2.1 ACK applied=true] FAIL - applied=${ack_applied} (期望 True)"
        record_result "2.1" "ACK applied=true" "FAIL" "applied=${ack_applied}"
        overall_pass=false
    fi

    # 校验 2：ACK version 与客户端本地 version 一致
    if [[ -n "$ack_version" && "$ack_version" == "$client_version" ]]; then
        log_info "✅ [用例 2.2 ACK version 一致] PASS - ACK version=${ack_version} == 客户端 version=${client_version}"
        record_result "2.2" "ACK version 一致" "PASS" "ack=${ack_version},client=${client_version}"
    else
        log_error "❌ [用例 2.2 ACK version 一致] FAIL - ACK version=${ack_version} != 客户端 version=${client_version}"
        record_result "2.2" "ACK version 一致" "FAIL" "ack=${ack_version},client=${client_version}"
        overall_pass=false
    fi

    # 校验 3：ACK md5 与客户端本地 md5 一致
    if [[ -n "$ack_md5" && "$ack_md5" == "$client_md5" ]]; then
        log_info "✅ [用例 2.3 ACK md5 一致] PASS - ACK md5=${ack_md5} == 客户端 md5=${client_md5}"
        record_result "2.3" "ACK md5 一致" "PASS" "ack=${ack_md5},client=${client_md5}"
    else
        log_error "❌ [用例 2.3 ACK md5 一致] FAIL - ACK md5=${ack_md5} != 客户端 md5=${client_md5}"
        record_result "2.3" "ACK md5 一致" "FAIL" "ack=${ack_md5},client=${client_md5}"
        overall_pass=false
    fi

    # ==================== 结果汇总 ====================
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║          配置生效查询验证结果汇总                 ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "  配置文件:        ${NAMESPACE}/${FILE_GROUP}/${FILE_NAME}"
    echo "  客户端 clientID: ${client_id}"
    echo "  客户端 version:  ${client_version}"
    echo "  客户端 md5:      ${client_md5}"
    echo "  ACK applied:      ${ack_applied}"
    echo "  ACK version:      ${ack_version}"
    echo "  ACK md5:          ${ack_md5}"
    echo ""
    echo "  用例明细:"
    awk -F',' 'NR>1 { printf "    [%s] %s: %s (%s)\n", $2, $3, $4, $5 }' "$RESULT_FILE"
    echo ""
    echo "  详细结果 CSV:    ${RESULT_FILE}"
    echo "  客户端日志:      ${LOG_DIR}/client.log"
    echo ""

    if [[ "$overall_pass" == "true" ]]; then
        echo -e "${GREEN}验证结论: ✅ 配置生效查询功能验证通过${NC}"
        echo -e "${GREEN}  - 客户端通过 WatchClientEvents 长连接响应服务端配置生效查询${NC}"
        echo -e "${GREEN}  - ACK 携带的 version/md5 与客户端本地生效配置一致${NC}"
        echo -e "${GREEN}  - applied=true 确认客户端正在监听该配置文件${NC}"
    else
        echo -e "${YELLOW}验证结论: ⚠️ 部分用例未通过，请对照上述明细与日志排查${NC}"
        echo -e "${YELLOW}  常见原因:${NC}"
        echo -e "${YELLOW}  1. 服务端未更新含 WatchClientEvents 逻辑的版本${NC}"
        echo -e "${YELLOW}  2. maintain 端口或鉴权 token 配置错误${NC}"
        echo -e "${YELLOW}  3. 客户端未订阅该配置文件 (检查 /config 的 version/md5 非空)${NC}"
        echo -e "${YELLOW}  4. WatchClientEvents 长连接未建立 (查 client.log 是否有 watcher 启动日志)${NC}"
    fi
    echo ""
}

main "$@"
