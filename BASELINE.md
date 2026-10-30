# 前端基线说明

本项目前端壳层复制自 `C:\Users\Administrator\Desktop\智能运维系统`，只做裁剪，不在 M0 升级框架版本。

- 基线提交：`8a7fded134aa3c5f30e4f68a539e85d17895c40f`
- 复制内容：Vue 3 + Vite 5 构建链、Ant Design Vue 主题与暗色模式、无边框窗口 `WindowControls`、布局与页面样式、`src/api/request.ts` 的请求封装与静默会话思路。
- 未复制：`workbench`、`hosts`、`devices`、`import` 等运维业务模块，xterm、docx、autostart 相关依赖与代码。

## 依赖差异

保留：`vue`、`vue-router`、`pinia`、`ant-design-vue`、`@ant-design/icons-vue`、`lucide-vue-next`、`axios`、`dayjs`、`@tauri-apps/api`。

新增：`echarts`（统计图表）。

删除：`@xterm/xterm`、`@xterm/addon-fit`、`docx`、`@tauri-apps/plugin-autostart`。

## 改造点

- 路由改为题库业务页（概览 / 题库 / 练习 / 考试 / 错题本 / 统计 / 设置），答题页独立于 `BasicLayout`。
- 登录改为本机静默会话：引擎仅监听 127.0.0.1，前端启动时自动换取 7 天 token。
- API 地址在打包运行时由 Tauri 注入（引擎端口为动态分配），开发期走 Vite 代理到 54310。
- 关闭窗口默认最小化到托盘，可在设置页改为退出程序。
