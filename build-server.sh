#!/usr/bin/env bash
# 服务端 Linux 二进制交叉编译。
# 版本号从 tauri.conf.json 读取并注入，避免再次出现「关于」页显示 dev。
# 用法：./build-server.sh [输出路径]，默认 dist-server/quizbox-engine
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
conf="$root/src-tauri/tauri.conf.json"

ver=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$conf" | head -n1)
if [ -z "$ver" ]; then
  echo "无法从 $conf 读取 version" >&2
  exit 1
fi

out="${1:-$root/dist-server/quizbox-engine}"
mkdir -p "$(dirname "$out")"

cd "$root/engine"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct \
  go build -trimpath -ldflags "-s -w -X main.version=$ver" -o "$out" ./cmd/engine

echo "built: $out (version $ver)"
