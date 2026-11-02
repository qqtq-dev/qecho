#!/usr/bin/env bash
# 题鸣（Qecho）网页版一键安装
#
# 用法（root 执行）：
#   ./install.sh                 # 默认装到 /opt/quizbox，监听 0.0.0.0:80
#   ./install.sh --port 3000     # 换端口
#   ./install.sh --addr 127.0.0.1 --port 54310   # 只监听本机，交给 Nginx
#
# 可重复执行：升级时把新的 quizbox-engine / dist 放到本目录再跑一次即可，
# 不会碰数据目录（默认 /var/lib/quizbox）。
set -euo pipefail

PREFIX="${PREFIX:-/opt/quizbox}"
DATA_DIR="${DATA_DIR:-/var/lib/quizbox}"
SERVICE="${SERVICE:-qecho}"
RUN_USER="${RUN_USER:-qecho}"
PORT=80
ADDR=0.0.0.0

while [ $# -gt 0 ]; do
  case "$1" in
    --port) PORT="$2"; shift 2 ;;
    --addr) ADDR="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) echo "未知参数：$1" >&2; exit 1 ;;
  esac
done

if [ "$(id -u)" != "0" ]; then
  echo "请用 root 执行（sudo ./install.sh）" >&2
  exit 1
fi

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "$HERE/quizbox-engine" ] || { echo "缺少 quizbox-engine" >&2; exit 1; }
[ -f "$HERE/dist/index.html" ] || { echo "缺少 dist/index.html" >&2; exit 1; }

# 包里按理已经带执行位（打包时在归档内修正过）；这里再补一次，覆盖手工拷贝的场景。
# 补不上就直接报错：以前是 || true 吞掉失败，结果 systemd 启动时才报 203/EXEC。
if [ ! -x "$HERE/quizbox-engine" ]; then
  chmod 755 "$HERE/quizbox-engine" 2>/dev/null || true
fi
if [ ! -x "$HERE/quizbox-engine" ]; then
  echo "quizbox-engine 没有执行权限，chmod 也无法修复（是否解压在 noexec 分区？）" >&2
  exit 1
fi
VER="$("$HERE/quizbox-engine" --version 2>/dev/null | tr -d '\r' | tail -1 || true)"
echo "== 安装题鸣网页版 ${VER:-未知版本} → $PREFIX (监听 $ADDR:$PORT)"

echo "== 1/5 创建运行用户"
if ! id -u "$RUN_USER" >/dev/null 2>&1; then
  useradd -r -s /usr/sbin/nologin -d "$PREFIX" "$RUN_USER" 2>/dev/null \
    || useradd -r -s /sbin/nologin -d "$PREFIX" "$RUN_USER"
fi

echo "== 2/5 部署程序文件"
systemctl stop "$SERVICE" 2>/dev/null || true
install -d -o "$RUN_USER" -g "$RUN_USER" "$PREFIX"
install -m 755 "$HERE/quizbox-engine" "$PREFIX/quizbox-engine"
rm -rf "$PREFIX/dist.new"
cp -r "$HERE/dist" "$PREFIX/dist.new"
rm -rf "$PREFIX/dist"
mv "$PREFIX/dist.new" "$PREFIX/dist"
chown -R "$RUN_USER:$RUN_USER" "$PREFIX"

echo "== 3/5 准备数据目录（已存在则保留）"
install -d -o "$RUN_USER" -g "$RUN_USER" "$DATA_DIR"
chown -R "$RUN_USER:$RUN_USER" "$DATA_DIR"

echo "== 4/5 写入 systemd 服务"
cat > "/etc/systemd/system/$SERVICE.service" <<UNIT
[Unit]
Description=Qecho quizbox engine (web)
After=network.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
WorkingDirectory=$PREFIX
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=$PREFIX/quizbox-engine --host $ADDR --port $PORT --data-dir $DATA_DIR --web-dir $PREFIX/dist
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

# RHEL 系开启 SELinux 时，/opt 下的可执行文件默认标签可能被拦截
if command -v selinuxenabled >/dev/null 2>&1 && selinuxenabled; then
  chcon -t bin_t "$PREFIX/quizbox-engine" 2>/dev/null || true
fi

systemctl daemon-reload
systemctl enable --now "$SERVICE"

echo "== 5/5 自检"
sleep 2
if systemctl is-active --quiet "$SERVICE"; then
  code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/" || true)
  echo "   服务状态：active"
  echo "   首页 HTTP：${code:-无响应}"
else
  echo "   服务未启动，最近日志："
  journalctl -u "$SERVICE" -n 20 --no-pager || true
  exit 1
fi

if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  if [ "$ADDR" != "127.0.0.1" ]; then
    firewall-cmd --permanent --add-port="$PORT/tcp" >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
    echo "   防火墙已放行 $PORT/tcp"
  fi
fi

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
cat <<TIP

安装完成。
  程序：$PREFIX/quizbox-engine + $PREFIX/dist
  数据：$DATA_DIR（SQLite 与备份，升级时不要删）
  服务：systemctl {start|stop|restart|status} $SERVICE

浏览器打开：http://${IP:-<本机IP>}$([ "$PORT" = "80" ] || echo ":$PORT")
首次访问请立刻注册，第一个注册的账号自动成为管理员。
TIP
