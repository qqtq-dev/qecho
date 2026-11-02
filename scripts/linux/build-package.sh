#!/usr/bin/env bash
# Linux 安装包构建
#
# 用法（在 Ubuntu 24.04 容器内、仓库根目录执行）：
#   scripts/linux/build-package.sh deb
#   scripts/linux/build-package.sh rpm
#   scripts/linux/build-package.sh appimage
#
# 前提：先跑过 scripts/linux/prepare-env.sh，且 src-tauri/binaries/ 下有
#       engine-x86_64-unknown-linux-gnu（用 build-server.sh 生成后改名）。
set -euo pipefail

export PATH="$HOME/.cargo/bin:$PATH"
export APPIMAGE_EXTRACT_AND_RUN=1

TARGET="${1:-deb}"
# GitHub 工具链下载策略：
#   - 容器里配了 HTTP(S)_PROXY → 先直连 GitHub（curl 自动走代理），失败再套镜像；
#   - 没配代理 → 先套 GH_PROXY 镜像，失败再直连；
#   - GH_PROXY= （置空）→ 只直连，不套镜像。
GH_PROXY="${GH_PROXY-https://ghfast.top/}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CACHE="$HOME/.cache/tauri"

log() { echo "=== [$(date +%H:%M:%S)] $* ==="; }

# fetch_github <url> <输出文件>：按上面的策略依次尝试候选地址
fetch_github() {
  local url="$1" out="$2" u
  local urls=()
  if [ -n "${HTTPS_PROXY:-${https_proxy:-}}" ] || [ -n "${HTTP_PROXY:-${http_proxy:-}}" ]; then
    urls+=("$url")
  fi
  [ -n "$GH_PROXY" ] && urls+=("${GH_PROXY}${url}")
  [ ${#urls[@]} -eq 0 ] && urls+=("$url")
  for u in "${urls[@]}"; do
    log "下载 $u"
    if curl -fL --retry 2 --retry-delay 2 -m 600 -o "$out" "$u"; then
      return 0
    fi
    rm -f "$out"
  done
  echo "工具链下载失败：$url" >&2
  return 1
}

cd "$ROOT"

log "同步 pnpm 版本"
npm install -g pnpm@12.6.0 --silent

log "安装前端依赖"
pnpm install --frozen-lockfile

case "$TARGET" in
  deb|rpm)
    log "编译并打包 $TARGET"
    npx tauri build --bundles "$TARGET"
    log "产物"
    find "$ROOT/src-tauri/target/release/bundle" -maxdepth 2 -type f \
      \( -name '*.deb' -o -name '*.rpm' \) -printf '%p  %kKB\n'
    ;;

  appimage)
    # 优先走 tauri 自带 bundler：它会自己拉 AppRun、linuxdeploy、linuxdeploy-plugin-appimage，
    # 实测这几个下载认 HTTP(S)_PROXY，配了代理就能一次成功，产物自包含（webkit2gtk 等库都在里面）。
    if [ -n "${HTTPS_PROXY:-${https_proxy:-}}" ] || [ -n "${HTTP_PROXY:-${http_proxy:-}}" ]; then
      log "有代理，用 tauri 自带 bundler 打 AppImage"
      if npx tauri build --bundles appimage; then
        log "产物"
        find "$ROOT/src-tauri/target/release/bundle/appimage" -maxdepth 1 -name '*.AppImage' \
          -printf '%p  %kKB\n'
        exit 0
      fi
      log "原生打包失败，退回手动 AppDir"
    fi

    # 没代理时 bundler 拉不到 GitHub 工具链，改为自己构造 AppDir 再调 linuxdeploy。
    log "编译（不打包）"
    npx tauri build --no-bundle

    mkdir -p "$CACHE"
    log "准备 linuxdeploy 工具链"
    [ -s "$CACHE/linuxdeploy-x86_64.AppImage" ] || \
      fetch_github "https://github.com/tauri-apps/binary-releases/releases/download/linuxdeploy/linuxdeploy-x86_64.AppImage" \
        "$CACHE/linuxdeploy-x86_64.AppImage"
    [ -s "$CACHE/linuxdeploy-plugin-gtk.sh" ] || \
      fetch_github "https://raw.githubusercontent.com/tauri-apps/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh" \
        "$CACHE/linuxdeploy-plugin-gtk.sh"
    chmod +x "$CACHE/linuxdeploy-x86_64.AppImage" "$CACHE/linuxdeploy-plugin-gtk.sh"

    log "构造 AppDir"
    APPDIR="$ROOT/build-appimage/Qecho.AppDir"
    rm -rf "$APPDIR"
    mkdir -p "$APPDIR/usr/bin" \
      "$APPDIR/usr/share/icons/hicolor/128x128/apps" \
      "$APPDIR/usr/share/icons/hicolor/256x256/apps" \
      "$APPDIR/usr/share/applications"

    cp "$ROOT/src-tauri/target/release/quizbox-desktop" "$APPDIR/usr/bin/"
    cp "$ROOT/src-tauri/binaries/engine-x86_64-unknown-linux-gnu" "$APPDIR/usr/bin/engine"
    chmod +x "$APPDIR/usr/bin/"*
    cp "$ROOT/src-tauri/icons/128x128.png"    "$APPDIR/usr/share/icons/hicolor/128x128/apps/quizbox-desktop.png"
    cp "$ROOT/src-tauri/icons/128x128@2x.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/quizbox-desktop.png"

    cat > "$APPDIR/usr/share/applications/quizbox-desktop.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Qecho
Name[zh_CN]=题鸣
Comment=单机题库练习与考试
Exec=quizbox-desktop
Icon=quizbox-desktop
Terminal=false
Categories=Education;
EOF

    log "调用 linuxdeploy"
    export PATH="$CACHE:$PATH"
    "$CACHE/linuxdeploy-x86_64.AppImage" \
      --appdir "$APPDIR" \
      --plugin gtk \
      --desktop-file "$APPDIR/usr/share/applications/quizbox-desktop.desktop" \
      --output appimage

    log "产物"
    find "$ROOT" -maxdepth 1 -name '*.AppImage' -printf '%p  %kKB\n'
    ;;

  *)
    echo "未知目标：$TARGET（可选 deb / rpm / appimage）" >&2
    exit 1
    ;;
esac
