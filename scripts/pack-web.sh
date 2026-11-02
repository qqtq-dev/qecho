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
# 这里的 -m 只在 Linux/macOS 上真生效；Windows 上会被忽略（见下方权限修正）
install -m 755 "$root/dist-server/quizbox-engine" "$stage/quizbox-engine"
cp -r "$root/dist" "$stage/dist"
install -m 755 "$root/scripts/web/install.sh" "$stage/install.sh"
install -m 644 "$root/scripts/web/README.txt" "$stage/README.txt"
sed -i "s/__VERSION__/$ver/g" "$stage/README.txt"

top="quizbox-web-$ver"
out="$root/dist-server/Qecho-$ver-web-linux-amd64.tar.gz"
tar -czf "$out" -C "$(dirname "$stage")" "$top"

# 包内可执行位只能在归档里改：Windows/MSYS 上给无扩展名的 ELF 设 +x 会静默失效
# （chmod 返回 0，stat 仍是 644），install -m 755 同样无效。tar-mode.py 直接改写
# 归档里的 mode 字段并回读校验，所以产物与打包平台无关；校验不过会非 0 退出。
PY="$(command -v python || command -v python3 || command -v py || true)"
[ -n "$PY" ] || { echo "需要 python 来修正包内可执行位" >&2; exit 1; }
"$PY" "$root/scripts/tar-mode.py" "$out" \
  "0755=$top/quizbox-engine" \
  "0755=$top/install.sh"

echo "== 产物：$out"
ls -lh "$out"
tar -tvzf "$out" | grep -E "($top/quizbox-engine|$top/install.sh)\$"
sha256sum "$out"
