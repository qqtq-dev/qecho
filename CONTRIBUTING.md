# 贡献指南

感谢你对 Qecho（题鸣）的关注。这份文档说明如何提 Issue、改代码和提交 PR。

## 提 Issue

- **Bug**：请说明操作系统与版本（Windows / Linux 发行版、桌面版还是网页版）、Qecho 版本号（设置页或 `release/version.json` 可查）、复现步骤、期望结果与实际结果。涉及界面问题的请附图。
- **功能建议**：先描述使用场景和当前的不便，再说你希望的方案。不必先写代码。
- **安全问题**：**不要**开公开 Issue。请通过 Gitee 私信或仓库主页的联系方式告知，确认修复后再公开细节。

## 开发环境

| 工具 | 版本要求 |
| --- | --- |
| Node | 22+（实测 v22.22.2） |
| pnpm | 12 |
| Go | 1.23+ |
| Rust | stable-msvc + VS C++ Build Tools（仅打包桌面端需要） |

前端与引擎的开发运行方式见 [README.md](README.md) 的「开发运行」一节。

## 分支与提交

- 从 `main` 切出功能分支，命名 `feat/简短描述`、`fix/简短描述`、`docs/简短描述`。
- 提交信息沿用仓库现有风格，使用 Conventional Commits 前缀：
  - `feat(scope): 新增功能`
  - `fix(scope): 修复问题`
  - `refactor(scope): 重构`
  - `docs: 文档`
  - `chore(release): 版本号与清单`
- scope 用模块名，如 `ui`、`exam`、`ai`、`build`、`i18n`。
- 一个 PR 只做一件事，不要把无关的格式化改动混进来。

## 提交前自查

```bash
# 前端
pnpm install
pnpm build

# Go 引擎
cd engine && go vet ./... && go test ./...

# 桌面端（如需）
cargo check --manifest-path src-tauri/Cargo.toml
```

- 新增或修改词条时，`src/i18n/locales/` 下所有语言文件都要补齐，词条类型由 `zh-CN` 推导，漏译会在编译期报错。
- 涉及数据库结构变更时，走 `engine/internal/store` 的版本化嵌入迁移，不要直接改已有迁移脚本。
- 不要提交 `data/`、`*.db`、`.env`、`src-tauri/binaries/`、构建产物与个人数据。

## 代码风格

- 前端：Vue 3 `<script setup>` + TypeScript，组件放 `src/components`，页面放 `src/views`。
- 引擎：Go 标准库优先，错误逐层返回，不要 `panic`。注释用中文，与现有代码一致。
- 保持现有文件组织，不要为了小改动大范围移动文件。

## 许可

提交的代码将按本仓库的 [MIT License](LICENSE) 授权。
