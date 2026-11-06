# Qecho Releases
![输入图片说明](image.png)

题鸣（Qecho）的版本发布仓库，只放版本清单与安装包，**不存放源码**。

源码在 [gitee.com/qqtq/qecho](https://gitee.com/qqtq/qecho)（主仓库），GitHub 镜像见 [July-Camellia/qecho](https://github.com/July-Camellia/qecho)。

## 下载

最新安装包见 [Releases](https://gitee.com/qqtq/qecho-releases/releases/latest)。

| 平台 | 安装包 |
| --- | --- |
| Windows 安装版 | `Qecho_x.y.z_x64-setup.exe` |
| Windows 免安装版 | `Qecho-x.y.z-x64-portable.zip` |
| Debian / Ubuntu | `Qecho_x.y.z_amd64.deb` |
| RHEL / Kylin / 统信 | `Qecho-x.y.z-1.x86_64.rpm` |
| 通用 Linux | `Qecho_x.y.z_amd64.AppImage` |

> 发行版页面里的 `Source code (zip)` 与 `Source code (tar.gz)` 是 Gitee 按标签自动生成的压缩包，
> 里面只有本仓库的 `version.json` 和 `README.md`，**不是客户端源码**，请不要当作安装包下载。

## 版本清单

客户端启动后会读取本仓库根目录的 [`version.json`](./version.json)，与自身版本比对，发现新版本时在右上角铃铛里提示。

字段说明：

| 字段 | 说明 |
| --- | --- |
| `version` | 最新版本号，与 `tauri.conf.json` 保持一致 |
| `publishedAt` | 发布日期，`YYYY-MM-DD` |
| `notes` | 更新摘要，客户端最多显示三行 |
| `url` | 版本说明页地址 |
| `downloadUrl` | 下载页地址，缺省时用 `url` |
| `force` | 置 `true` 时客户端会标为重要更新 |

```json
{
  "version": "0.2.0",
  "publishedAt": "2026-11-02",
  "notes": "新增题库共享、用户权限管理与站内通知；答题卡改为四态配色。",
  "url": "https://gitee.com/qqtq/qecho-releases/releases/tag/v0.2.0",
  "downloadUrl": "https://gitee.com/qqtq/qecho-releases/releases/tag/v0.2.0",
  "force": false
}
```

修改 `version.json` 后客户端最长 30 分钟内生效，点开铃铛会立即刷新。
