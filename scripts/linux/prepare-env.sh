#!/usr/bin/env bash
# Linux 构建环境准备（在 Ubuntu 24.04 容器内以 root 执行一次）
#
# 说明：
#   - RHEL/Rocky 系没有 webkit2gtk-4.1，无法直接构建 Tauri 2；这里统一用 Ubuntu 24.04 容器。
#   - 基础镜像没有 ca-certificates，所以 apt 源先用 http（包本身有 GPG 签名校验，不影响安全性）。
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
export LANG=C.UTF-8

NODE_VERSION="${NODE_VERSION:-20.19.0}"
PNPM_VERSION="${PNPM_VERSION:-12.6.0}"

log() { echo "=== [$(date +%H:%M:%S)] $* ==="; }

log "配置 apt 源（阿里云，http）"
cat > /etc/apt/sources.list.d/ubuntu.sources <<'EOF'
Types: deb
URIs: http://mirrors.aliyun.com/ubuntu/
Suites: noble noble-updates noble-backports
Components: main restricted universe multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg

Types: deb
URIs: http://mirrors.aliyun.com/ubuntu/
Suites: noble-security
Components: main restricted universe multiverse
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
EOF

log "安装系统依赖"
apt-get update
apt-get install -y --no-install-recommends \
  build-essential pkg-config curl wget file ca-certificates xz-utils \
  libwebkit2gtk-4.1-dev libgtk-3-dev libayatana-appindicator3-dev \
  librsvg2-dev libssl-dev libxdo-dev patchelf

log "安装 Node ${NODE_VERSION}"
cd /tmp
curl -fsSL -o node.tar.xz "https://registry.npmmirror.com/-/binary/node/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz"
tar -xJf node.tar.xz -C /usr/local --strip-components=1
node -v && npm -v
npm config set registry https://registry.npmmirror.com

log "安装 pnpm ${PNPM_VERSION}"
npm install -g "pnpm@${PNPM_VERSION}" --silent
pnpm -v

log "安装 Rust（rsproxy 镜像）"
export RUSTUP_DIST_SERVER=https://rsproxy.cn
export RUSTUP_UPDATE_ROOT=https://rsproxy.cn/rustup
curl -fsSL https://rsproxy.cn/rustup-init.sh | sh -s -- -y --profile minimal --default-toolchain stable
export PATH="$HOME/.cargo/bin:$PATH"
rustc -V && cargo -V

mkdir -p "$HOME/.cargo"
cat > "$HOME/.cargo/config.toml" <<'EOF'
[source.crates-io]
replace-with = 'rsproxy-sparse'

[source.rsproxy-sparse]
registry = "sparse+https://rsproxy.cn/index/"

[registries.rsproxy]
index = "https://rsproxy.cn/crates.io-index"

[net]
git-fetch-with-cli = true
EOF

log "环境就绪"
