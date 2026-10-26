#!/usr/bin/env bash
# gen-mqtt-certs.sh —— MQTT 信令通道自签证书生成(CA + EMQX 服务端证书)
#
# 产物(backend/certs/emqx/):
#   ca.pem        自签 CA 证书 —— 分发给所有 core/agent 作为 file.ca_path
#   server.pem    EMQX SSL listener 服务端证书
#   server.key    服务端私钥(仅 EMQX 容器持有)
#   serial.srl / *.csr 为过程产物,脚本结束清理。
#
# 用法(在 backend/ 下):
#   ./scripts/maintenance/gen-mqtt-certs.sh              # 已存在则跳过
#   FORCE=1 ./scripts/maintenance/gen-mqtt-certs.sh      # 推翻重建
#
# broker 主机名/IP 通过 CERT_SAN 扩展(逗号分隔),默认覆盖 dev 场景:
#   CERT_SAN="DNS:emqx,DNS:localhost,IP:127.0.0.1" ./scripts/maintenance/gen-mqtt-certs.sh
#
# 证书就位后:
#   1) docker compose -f docker-compose.libs.yaml up -d emqx(SLL listener 挂载本目录)
#   2) core/agent 的 server.yaml:endpoint 改 ssl://<broker>:8883,
#      tls.file.ca_path 指向 ca.pem(dev 也可临时 insecure_skip_verify: true)
set -euo pipefail

OUT_DIR="$(cd "$(dirname "$0")/../.." && pwd)/certs/emqx"
CA_KEY="$OUT_DIR/ca.key"
CA_PEM="$OUT_DIR/ca.pem"
SERVER_KEY="$OUT_DIR/server.key"
SERVER_PEM="$OUT_DIR/server.pem"
SAN="${CERT_SAN:-DNS:emqx,DNS:localhost,IP:127.0.0.1}"
DAYS_CA="${DAYS_CA:-3650}"
DAYS_SERVER="${DAYS_SERVER:-825}"

if [[ -f "$CA_PEM" && -f "$SERVER_PEM" && -f "$SERVER_KEY" && "${FORCE:-0}" != "1" ]]; then
  echo "[gen-mqtt-certs] 证书已存在,跳过(FORCE=1 重生成): $OUT_DIR"
  exit 0
fi

command -v openssl >/dev/null 2>&1 || { echo "[gen-mqtt-certs] 需要 openssl" >&2; exit 1; }

# 双重坑:Git Bash(MSYS)默认把 "-subj /CN=..." 当路径转成 "C:/Program Files/Git/...",
# 而原生 Windows 版 openssl 又不认 "/d/..." POSIX 路径。统一策略——禁掉 MSYS 自动
# 转换,文件路径经 cygpath -m 显式转 Windows 形式(无 cygpath 的平台原样传)。
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL="*"
wpath() { command -v cygpath >/dev/null 2>&1 && cygpath -m "$1" || printf '%s' "$1"; }

mkdir -p "$OUT_DIR"
OUT_DIR_W="$(wpath "$OUT_DIR")"

echo "[gen-mqtt-certs] 生成自签 CA(有效期 ${DAYS_CA} 天)..."
openssl req -x509 -newkey rsa:2048 -nodes -days "$DAYS_CA" \
  -keyout "$OUT_DIR_W/ca.key" -out "$OUT_DIR_W/ca.pem" \
  -subj "/CN=go-wind-ops MQTT CA/O=GoWind Ops" >/dev/null 2>&1
if [[ ! -s "$CA_PEM" ]]; then echo "[gen-mqtt-certs] CA 生成失败" >&2; exit 1; fi

echo "[gen-mqtt-certs] 生成 EMQX 服务端证书(SAN: $SAN)..."
openssl req -newkey rsa:2048 -nodes \
  -keyout "$OUT_DIR_W/server.key" -out "$OUT_DIR_W/server.csr" \
  -subj "/CN=emqx/O=GoWind Ops" >/dev/null 2>&1
# 扩展字段落盘:Git Bash 的进程替换给 openssl 一个 /proc/<pid>/fd/N 路径,读不了
printf "subjectAltName=%s\nextendedKeyUsage=serverAuth\n" "$SAN" > "$OUT_DIR/server.ext"
openssl x509 -req -in "$OUT_DIR_W/server.csr" \
  -CA "$OUT_DIR_W/ca.pem" -CAkey "$OUT_DIR_W/ca.key" -CAcreateserial \
  -days "$DAYS_SERVER" \
  -extfile "$OUT_DIR_W/server.ext" \
  -out "$OUT_DIR_W/server.pem" >/dev/null 2>&1
rm -f "$OUT_DIR/server.ext"
if [[ ! -s "$SERVER_PEM" ]]; then echo "[gen-mqtt-certs] 服务端证书生成失败" >&2; exit 1; fi

rm -f "$OUT_DIR/server.csr" "$OUT_DIR/ca.srl"

echo "[gen-mqtt-certs] 完成: $OUT_DIR"
echo "[gen-mqtt-certs] 注意: ca.key 只用于签发,无须分发;server.key 仅 EMQX 容器持有"
