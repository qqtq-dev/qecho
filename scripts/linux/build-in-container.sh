#!/usr/bin/env bash
# 在任意装了 podman 的 Linux 上构建 Qecho 的 Linux 安装包
#
# 用法：
#   scripts/linux/build-in-container.sh deb
#   scripts/linux/build-in-container.sh rpm
#   scripts/linux/build-in-container.sh appimage
#
# 首次运行会拉取 Ubuntu 24.04 镜像并准备编译环境（约 10~15 分钟），
# 之后 commit 成 qecho-builder:base，再次构建直接复用。
set -euo pipefail

TARGET="${1:-deb}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE="qecho-builder:base"
UBUNTU="docker.m.daocloud.io/library/ubuntu:24.04"

PROXY_ENV=()
for v in HTTP_PROXY HTTPS_PROXY http_proxy https_proxy NO_PROXY no_proxy; do
  [ -n "${!v:-}" ] && PROXY_ENV+=(--env "$v")
done

command -v podman >/dev/null || { echo "需要 podman：dnf install -y podman" >&2; exit 1; }

if ! podman image exists "$IMAGE"; then
  echo "=== 首次构建：准备 Ubuntu 24.04 环境 ==="
  podman pull "$UBUNTU"
  podman tag "$UBUNTU" ubuntu:24.04
  podman rm -f qecho-prep >/dev/null 2>&1 || true
  podman run --name qecho-prep "${PROXY_ENV[@]}" -v "$SRC":/work:Z -w /work ubuntu:24.04 \
    bash /work/scripts/linux/prepare-env.sh
  podman commit qecho-prep "$IMAGE"
  podman rm -f qecho-prep
fi

echo "=== 构建 $TARGET ==="
podman run --rm "${PROXY_ENV[@]}" -v "$SRC":/work:Z -w /work "$IMAGE" \
  bash "/work/scripts/linux/build-package.sh" "$TARGET"
