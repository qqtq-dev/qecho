# Qecho

> Qecho（题鸣）—— 单机题库练习与考试工具，不依赖服务器，数据全部保存在本机。
> Tauri 2 壳 + Vue 3 前端 + Go 引擎（sidecar）+ SQLite。

完整设计见 [docs/题鸣详细设计文档.md](docs/题鸣详细设计文档.md)。

## 当前进度

- 前端壳层：无边框窗口、托盘、深色模式、侧边导航、概览 / 题库 / 练习 / 考试 / 错题本 / 统计 / 设置页面。
- 本地引擎：`/health` 探活、静默本机会话（7 天 token）、系统状态、优雅关闭；SQLite WAL 与版本化嵌入迁移。
- 题库管理：题库与题目增删改查，单选/多选、2–8 个选项、答案、解析、难度、题图地址。
- 题库导出：题库详情页一键导出为 Excel（题型/题目/题干图片/选项/答案/解析/难度），桌面端弹系统保存对话框选位置，网页版直接下载；导出文件可直接再导入。
- 题库导入：Excel（.xlsx/.xlsm）与 CSV 上传后先预览（列映射、可导入/重复/错误统计、样例与错误明细），确认后批量入库；按题干哈希跳过重复，整批可撤销（7 天内、未编辑的题目）。
- 外联考试：按题库抽题生成考试，设置密码、时间窗、时长与乱序；生成局域网链接 `http://<IP>:<端口>/e/<token>`，参与者手机/电脑浏览器直接答题，自动保存、断线续答、服务端判分，主机端汇总成绩。
- 桌面集成：Tauri 启动时拉起 sidecar，动态端口通过 `ENGINE_PORT` 回传；sidecar 监听 `0.0.0.0` 以支持外联考试；关闭窗口默认最小化到托盘；分享面板可一键申请防火墙放行。
- 账号与权限：注册（图形验证码）/ 登录 / 退出，首个注册用户自动成为管理员；题库分公共（管理员维护）与个人题库，写操作按归属校验，考试仅创建者可管理。
- 练习：按题库或错题本抽题，服务端会话 + 逐题判分，支持上一题 / 下一题、交卷与重练；答题结果实时同步错题本与统计。
- 错题本：答错自动收录，连续答对 2 次标记为已掌握；支持待复习 / 已掌握筛选、分页、展开查看选项与解析、单题重练与移出。
- 网页版部署：同一引擎加 `--host/--port/--web-dir` 即可托管网页版，支持 systemd 与 Nginx HTTPS，详见 [docs/服务端部署.md](docs/服务端部署.md)。
- 待接入：统计口径、图片上传（当前题图支持填写链接）。

## 开发运行

```bash
# 终端 1：引擎
cd engine
go run ./cmd/engine --data-dir ../data --port 54310

# 终端 2：前端（http://127.0.0.1:54311，/api 自动代理到 54310）
pnpm install
pnpm dev
```

`pnpm tauri dev` 的引擎在开发模式下固定用 54310，浏览器与桌面窗口共用同一引擎和数据库；打包后恢复动态端口。

桌面模式：

```bash
pnpm tauri dev
```

## 打包

```powershell
.\build.ps1
```

`build.ps1` 依次执行 `build-engine.ps1`（Go 引擎 → `src-tauri/binaries/`）、`pnpm install --frozen-lockfile`、`pnpm tauri build`。安装包输出到 `src-tauri/target/release/bundle/nsis/`。

## 数据目录

- 打包运行：`%APPDATA%\com.quizbox.desktop`（由 Tauri 传入引擎）
- 开发运行：项目内 `data/`（通过 `--data-dir` 指定）

## 目录结构

```text
├── src/                 # Vue 3 前端（Vue Router / Pinia / Ant Design Vue）
│   ├── api/             #   axios 封装、引擎与鉴权接口
│   ├── components/      #   PageHeader / MetricTile / WindowControls
│   ├── layouts/         #   BasicLayout（顶栏 + 侧边菜单）
│   ├── router/          #   hash 路由
│   ├── stores/          #   Pinia（引擎健康、本机会话）
│   └── views/           #   各业务页面
├── src-tauri/           # Tauri 2 壳（sidecar 管理、托盘、窗口）
├── engine/              # Go 引擎（stdlib net/http + modernc.org/sqlite）
│   ├── cmd/engine/
│   └── internal/        #   api / authx / response / store
├── docs/                # 设计文档
└── BASELINE.md          # 前端复用基线与依赖差异
```

## 环境

- Node 24 + pnpm 12
- Go 1.23+
- Rust stable-msvc + VS C++ Build Tools（打包桌面端）
