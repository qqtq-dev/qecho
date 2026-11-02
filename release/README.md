# Qecho Releases

题鸣（Qecho）的版本发布仓库，只放版本清单与安装包，**源码不在这个仓库**。

## 下载

最新安装包见 [Releases](https://gitee.com/qqtq/qecho-releases/releases)。

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
  "notes": "练习页解析框支持内部滚动；新增版本更新提醒。",
  "url": "https://gitee.com/qqtq/qecho-releases",
  "downloadUrl": "https://gitee.com/qqtq/qecho-releases/releases",
  "force": false
}
```

修改 `version.json` 后客户端最长 30 分钟内生效，点开铃铛会立即刷新。
