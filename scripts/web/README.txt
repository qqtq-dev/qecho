题鸣（Qecho）网页版 __VERSION__ · linux-amd64
========================================

这是什么
  服务端部署包。解压后运行 install.sh，即可用浏览器多人访问。
  不是桌面客户端，不需要图形界面、不需要 Go/Node、不依赖 glibc 与 libGL。

包内文件
  quizbox-engine   引擎（Go 静态可执行，x86_64）
  dist/            前端构建产物
  install.sh       一键安装脚本
  README.txt       本文件

安装（root）
  tar xzf Qecho-__VERSION__-web-linux-amd64.tar.gz
  cd quizbox-web-__VERSION__
  sudo ./install.sh                      # 默认 0.0.0.0:80
  sudo ./install.sh --port 3000          # 换端口
  sudo ./install.sh --addr 127.0.0.1 --port 54310   # 交给 Nginx 反向代理

安装后
  程序  /opt/quizbox/quizbox-engine + /opt/quizbox/dist
  数据  /var/lib/quizbox（quiz.db 与备份，升级时不要删）
  服务  systemctl status qecho

首次访问请立刻注册，第一个注册的账号自动成为管理员。

升级
  把新的 quizbox-engine 与 dist/ 覆盖到本目录，重跑 install.sh。
  数据目录不动；引擎启动时按 user_version 自动迁移，
  迁移前会把旧库备份到 <数据目录>/backups/pre-migrate-*.zip。

常用命令
  systemctl restart qecho
  journalctl -u qecho -n 50 --no-pager
  /opt/quizbox/quizbox-engine --verify-db /var/lib/quizbox/quiz.db
  sqlite3 /var/lib/quizbox/quiz.db ".backup '/var/backups/quizbox-$(date +%F).db'"
