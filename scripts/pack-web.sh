#!/usr/bin/env bash
# 打服务端网页版安装包：
#   引擎（linux/amd64 静态链接） + dist/ + install.sh + README
#   → dist-server/Qecho-<版本>-web-linux-amd64.tar.gz
#
# 用法：./scripts/pack-web.sh
# 前置：pnpm build 已产出 dist/（本脚本发现缺失会自动构建）
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
conf="$root/src-tauri/tauri.conf.json"

ver=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$conf" | head -n1)
[ -n "$ver" ] || { echo "无法从 $conf 读取 version" >&2; exit 1; }

if [ ! -f "$root/dist/index.html" ]; then
  echo "== dist 缺失，先构建前端"
  (cd "$root" && pnpm build)
fi

echo "== 交叉编译引擎（linux/amd64，版本 $ver）"
"$root/build-server.sh" "$root/dist-server/quizbox-engine"

stage="$(mktemp -d)/quizbox-web-$ver"
mkdir -p "$stage"
install -m 755 "$root/dist-server/quizbox-engine" "$stage/quizbox-engine"
cp -r "$root/dist" "$stage/dist"
install -m 755 "$root/scripts/web/install.sh" "$stage/install.sh"
install -m 644 "$root/scripts/web/README.txt" "$stage/README.txt"
sed -i "s/__VERSION__/$ver/g" "$stage/README.txt"

out="$root/dist-server/Qecho-$ver-web-linux-amd64.tar.gz"
tar -czf "$out" -C "$(dirname "$stage")" "quizbox-web-$ver"

echo "== 产物：$out"
ls -lh "$out"
sha256sum "$out"
