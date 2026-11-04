# Linux 桌面版打包

Windows 用 `build.ps1` 一条命令出 NSIS 安装包，Linux 这边情况不同，先看结论：

| 产物 | 适用系统 | 是否自包含 |
|---|---|---|
| `Qecho_0.2.5_amd64.deb` | Ubuntu 22.04+、Debian 12+、深度、优麒麟 | 否，依赖系统 `libwebkit2gtk-4.1-0`、`libgtk-3-0`、`libayatana-appindicator3-1` |
| `Qecho-0.2.5-1.x86_64.rpm` | 麒麟 V10（rpm 版）、统信 UOS（rpm 版）、Rocky/Fedora/CentOS 9+ | 否，同上 |
| `Qecho_0.2.5_amd64.AppImage` | 任意 x86_64 桌面发行版 | 基本自包含，WebKitGTK 等 178 个库全部打进去了，约 92 MB；显卡库（`libGL` 等）仍需系统提供，见下文 |

普通用户优先给 AppImage（双击即用、不用装依赖），其次是 deb。

## 为什么必须用 Ubuntu 容器

Tauri 2 依赖 `webkit2gtk-4.1` 和 `libsoup-3.0`。RHEL 9 系（Rocky、Alma、麒麟服务器版）只有 `webkit2gtk-4.0` + `libsoup2`，系统源和 EPEL 里都没有 4.1，所以在 Rocky 上直接 `cargo build` 一定失败：

```
Package webkit2gtk-4.1 was not found in the pkg-config search path
```

解决办法是在 Ubuntu 24.04 容器里编译。宿主机只要装了 podman 就行，不需要装 Rust、Node 或 GUI 依赖。

## 完整流程

前提：`src-tauri/binaries/engine-x86_64-unknown-linux-gnu` 存在。这是引擎 sidecar，由 `build-server.sh` 生成后改名得到：

```bash
./build-server.sh                                    # 输出 dist-server/quizbox-engine
cp dist-server/quizbox-engine src-tauri/binaries/engine-x86_64-unknown-linux-gnu
chmod +x src-tauri/binaries/engine-x86_64-unknown-linux-gnu
```

然后在 Linux 机器上（仓库根目录）：

```bash
# 首次：拉镜像 + 装依赖 + 装 Node/Rust，约 10~15 分钟
scripts/linux/build-in-container.sh deb

# 之后：复用 qecho-builder:base，单次约 3 分钟（Rust 增量编译）
scripts/linux/build-in-container.sh rpm
scripts/linux/build-in-container.sh appimage
```

产物位置：

```
src-tauri/target/release/bundle/deb/Qecho_0.2.5_amd64.deb
src-tauri/target/release/bundle/rpm/Qecho-0.2.5-1.x86_64.rpm
src-tauri/target/release/bundle/appimage/Qecho_0.2.5_amd64.AppImage   # 有代理走官方 bundler
Qecho-x86_64.AppImage                                               # 无代理退回手动方案
```

## 三个脚本各自做什么

| 脚本 | 运行位置 | 职责 |
|---|---|---|
| `scripts/linux/build-in-container.sh` | 宿主机 | 编排 podman：首次建镜像，之后直接跑构建 |
| `scripts/linux/prepare-env.sh` | 容器内，只需一次 | apt 源、编译依赖、Node 20、pnpm 12、Rust（rsproxy 镜像） |
| `scripts/linux/build-package.sh` | 容器内 | 安装依赖、编译，按参数打 deb / rpm / appimage |

镜像源都走了国内：apt → 阿里云，npm → npmmirror，crates → rsproxy，容器镜像 → `docker.m.daocloud.io`。

## AppImage：有代理就用官方 bundler

`tauri build --bundles appimage` 会下载**固定版本**的 AppRun、linuxdeploy、linuxdeploy-plugin-appimage，国内直连 GitHub 拿不到时报：

```
Downloading https://github.com/tauri-apps/binary-releases/releases/download/linuxdeploy-07333c6/linuxdeploy-x86_64.AppImage
failed to bundle project: `io: Network is unreachable (os error 101)`
```

但它的下载器认 `HTTP_PROXY` / `HTTPS_PROXY`（已实测）：**配了代理时 `build-package.sh appimage` 直接调官方 bundler**，产物在 `src-tauri/target/release/bundle/appimage/Qecho_0.2.5_amd64.AppImage`，约 92 MB，解包后 `usr/lib` 里有 178 个库（`libwebkit2gtk-4.1` 等都在），自包含。

没代理时才退回手动方案，产物是仓库根目录的 `Qecho-x86_64.AppImage`（约 92 MB）：

1. `tauri build --no-bundle` 只编译出 `quizbox-desktop`
2. 手动构造 AppDir（主程序、sidecar、图标、`.desktop`）
3. 用经镜像预置的 `linuxdeploy` + `--plugin gtk` 生成 AppImage

手动方案的镜像地址可用环境变量覆盖，默认 `https://ghfast.top/`：

```bash
GH_PROXY=https://gh-proxy.com/ scripts/linux/build-package.sh appimage
```

### 有 HTTP 代理时优先直连

镜像站（ghfast.top 之类）随时可能限速或挂掉，能直连 GitHub 就别绕。`build-in-container.sh` 会把宿主机的 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY`（大小写都认）原样传进容器，`build-package.sh` 里的 `fetch_github` 据此决定顺序：

| 环境 | 下载顺序 |
|---|---|
| 配了 `HTTPS_PROXY` | 先直连 GitHub（curl 自动走代理），失败再套镜像 |
| 没配代理 | 先套 `GH_PROXY` 镜像，失败再直连（一般直接失败） |
| `GH_PROXY=` 置空 | 只直连，不套镜像 |

代理地址必须是**容器能访问的**地址。代理跑在宿主机上时不能用 `127.0.0.1`，要写宿主机的局域网 IP（例如 `HTTPS_PROXY=http://192.168.40.1:7890`），并且代理程序要允许来自局域网的连接。

国内镜像（apt/npm/crates）不要走代理，加进 `NO_PROXY` 更稳：

```bash
export HTTPS_PROXY=http://192.168.40.1:7890
export NO_PROXY=localhost,127.0.0.1,.aliyun.com,.npmmirror.com,.rsproxy.cn
scripts/linux/build-in-container.sh appimage
```

容器内跑 AppImage 工具需要 `APPIMAGE_EXTRACT_AND_RUN=1`（脚本已设），因为没有 FUSE。

## AppImage 运行时报缺库

AppImage 打包了 WebKitGTK 等 178 个库，但**故意不打包显卡相关库**（`libGL` / `libEGL` / `libgbm` / `libdrm`）——这些必须和目标机的显卡驱动匹配，塞进镜像反而会崩。所以精简安装的 RHEL 系系统（OpenCloudOS、Rocky、Alma、麒麟服务器版）首次运行会报：

```
quizbox-desktop: error while loading shared libraries: libGL.so.1: cannot open shared object file: No such file or directory
```

补装即可，装完直接重跑，**不需要重新打包**：

```bash
# RHEL / CentOS / OpenCloudOS / Rocky / Alma / 麒麟（rpm 系）
sudo dnf install -y mesa-libGL mesa-libEGL mesa-libgbm libglvnd-glx libglvnd-egl

# Ubuntu / Debian / 深度 / 优麒麟（deb 系）
sudo apt install -y libgl1 libegl1 libgbm1
```

如果补完 GL 后接着报 `cannot open display` 或 `Failed to connect to display`，说明这台机器没有图形界面（云主机常见），AppImage 不是适用场景，得配 X11 转发或装 VNC/桌面环境。

rpm 系还有更省事的路：直接 `sudo dnf install ./Qecho-0.2.5-1.x86_64.rpm`，依赖交给包管理器自动补。前提是软件源里有 `webkit2gtk4.1`——RHEL 9.2+ / OpenCloudOS 9 有，CentOS 7、8 没有。

## 已知限制

**sidecar 装成了 `/usr/bin/engine`**。`bundle.externalBin` 写的是 `binaries/engine`，deb/rpm 会把它装到 `/usr/bin/engine`，这个名字太泛，理论上会和别的软件冲突。要改需要同时动三处：`tauri.conf.json` 的 `externalBin`、`src-tauri/src/lib.rs` 里的 sidecar 调用名、两个构建脚本里的文件名。暂时保留，等真的撞了再改。

**deb/rpm 不含引擎图标资源以外的运行时**。装完如果在最小化系统上起不来，补装依赖即可：

```bash
sudo apt install -y libwebkit2gtk-4.1-0 libgtk-3-0 libayatana-appindicator3-1   # deb 系
sudo dnf install -y webkit2gtk4.1 gtk3 libayatana-appindicator-gtk3            # rpm 系
```

**麒麟 / 统信要注意包类型**。这两家都有 deb 版和 rpm 版系统，发之前确认用户的系统版本，别给麒麟 V10 SP1（deb）发 rpm。

## 版本号

版本号统一来自 `src-tauri/tauri.conf.json` 的 `version`，三个包名都会跟着变。发版流程见 [版本发布.md](版本发布.md)。
