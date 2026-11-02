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
GH_PROXY="${GH_PROXY:-https://ghfast.top/}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CACHE="$HOME/.cache/tauri"

log() { echo "=== [$(date +%H:%M:%S)] $* ==="; }

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
    # tauri bundler 打 AppImage 时会从 GitHub 拉固定版本工具链，国内网络基本拿不到，
    # 因此这里自己构造 AppDir 再调 linuxdeploy，工具包经代理预置。
    log "编译（不打包）"
    npx tauri build --no-bundle

    mkdir -p "$CACHE"
    log "准备 linuxdeploy 工具链"
    [ -s "$CACHE/linuxdeploy-x86_64.AppImage" ] || \
      curl -fL --retry 2 -o "$CACHE/linuxdeploy-x86_64.AppImage" \
        "${GH_PROXY}https://github.com/tauri-apps/binary-releases/releases/download/linuxdeploy/linuxdeploy-x86_64.AppImage"
    [ -s "$CACHE/linuxdeploy-plugin-gtk.sh" ] || \
      curl -fL --retry 2 -o "$CACHE/linuxdeploy-plugin-gtk.sh" \
        "${GH_PROXY}https://raw.githubusercontent.com/tauri-apps/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh"
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
