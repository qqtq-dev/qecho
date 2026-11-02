#!/usr/bin/env python3
"""把一次构建的安装包发布到 Gitee 公开仓库的发行版里。

用法：
    export GITEE_TOKEN=<私人令牌>        # Gitee → 设置 → 私人令牌，勾 projects
    python scripts/publish-release.py 0.3.0

依赖：requests（pip install requests）。
"""

import argparse
import hashlib
import os
import sys
import time
from pathlib import Path

import requests

OWNER, REPO = "qqtq", "qecho-releases"
API = f"https://gitee.com/api/v5/repos/{OWNER}/{REPO}"
ROOT = Path(__file__).resolve().parent.parent
DIST_LINUX = ROOT / "dist-linux"
DIST_WIN = ROOT / "dist-windows"
NSIS = ROOT / "src-tauri" / "target" / "release" / "bundle" / "nsis"


def log(*args):
    print(f"[{time.strftime('%H:%M:%S')}]", *args, flush=True)


def collect(version):
    """按版本号找出本次要发布的文件，缺哪个就跳过哪个。"""
    candidates = [
        (NSIS / f"Qecho_{version}_x64-setup.exe", "Windows 安装版", "9 MB"),
        (DIST_WIN / f"Qecho-{version}-x64-portable.zip", "Windows 免安装版", "12 MB"),
        (DIST_LINUX / f"Qecho_{version}_amd64.deb", "Debian / Ubuntu", "12 MB"),
        (DIST_LINUX / f"Qecho-{version}-1.x86_64.rpm", "RHEL / Kylin", "12 MB"),
        (DIST_LINUX / f"Qecho_{version}_amd64.AppImage", "通用 Linux", "86 MB"),
    ]
    found = [(p, label) for p, label, _ in candidates if p.exists()]
    if not found:
        log("没有找到任何安装包，先跑构建脚本")
        sys.exit(1)
    for p, _ in found:
        log(f"打包 {p.name}  {p.stat().st_size / 1048576:.1f} MB")
    missing = [p.name for p, _, _ in candidates if not p.exists()]
    if missing:
        log("缺少（将不发布）：", ", ".join(missing))
    return found


def sha256(path, chunk=1 << 20):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(chunk), b""):
            h.update(block)
    return h.hexdigest()


def build_body(version, files):
    rows = []
    for path, label in files:
        rows.append(f"| {label} | `{path.name}` | {path.stat().st_size / 1048576:.1f} MB |")
    table = "\n".join(rows)
    tag = f"https://gitee.com/{OWNER}/{REPO}/releases/tag/v{version}"
    return f"""**Qecho（题鸣）{version}** —— 题库练习与考试工具，桌面端（Windows / Linux）与服务端网页版同源。

## 下载

| 平台 | 文件 | 大小 |
| --- | --- | --- |
{table}

Windows 需要 WebView2 运行时（Win11 与较新的 Win10 默认已装）。免安装版的数据存在 `%APPDATA%\\com.quizbox.desktop`。

服务端部署见主仓库 [docs/服务端部署.md](https://gitee.com/qqtq/qecho/blob/main/docs/服务端部署.md)。

## 校验值

见附件 `SHA256SUMS.txt`；下载地址前缀为 `{tag}`。
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("version", help="版本号，如 0.3.0")
    args = ap.parse_args()
    version = args.version.lstrip("vV")
    token = os.environ.get("GITEE_TOKEN", "").strip()
    if not token:
        log("缺少 GITEE_TOKEN 环境变量")
        sys.exit(1)

    files = collect(version)

    # 1. 建发行版（已存在就复用）
    releases = requests.get(f"{API}/releases", params={"access_token": token}, timeout=30).json()
    rel = next((r for r in releases if r.get("tag_name") == f"v{version}"), None)
    if rel:
        log("发行版已存在，复用 id =", rel["id"])
    else:
        resp = requests.post(
            f"{API}/releases",
            data={
                "access_token": token,
                "tag_name": f"v{version}",
                "name": f"Qecho {version}",
                "body": build_body(version, files),
                "target_commitish": "master",
                "prerelease": "false",
            },
            timeout=60,
        )
        if resp.status_code not in (200, 201):
            log("建发行版失败", resp.status_code, resp.text[:400])
            sys.exit(1)
        rel = resp.json()
        log("发行版已创建 id =", rel["id"])
    rid = rel["id"]

    # 2. 逐个上传，同名的跳过
    existing = {a["name"] for a in (rel.get("assets") or [])}
    sums = []
    for path, _ in files:
        sums.append(f"{sha256(path)}  {path.name}")
        if path.name in existing:
            log(f"跳过 {path.name}（已存在）")
            continue
        log(f"上传 {path.name} ...")
        started = time.time()
        with open(path, "rb") as fh:
            resp = requests.post(
                f"{API}/releases/{rid}/attach_files",
                data={"access_token": token},
                files={"file": (path.name, fh)},
                timeout=(30, 1800),
            )
        if resp.status_code not in (200, 201):
            log("  失败", resp.status_code, resp.text[:400])
            sys.exit(1)
        log(f"  完成 {time.time() - started:.0f}s -> {resp.json().get('browser_download_url')}")

    # 3. 校验值文件
    sums_text = "\n".join(sorted(sums)) + "\n"
    sums_path = ROOT / "dist-linux" / "SHA256SUMS.txt"
    sums_path.write_text(sums_text, encoding="utf-8", newline="\n")
    if "SHA256SUMS.txt" in existing:
        log("跳过 SHA256SUMS.txt（已存在）")
    else:
        with open(sums_path, "rb") as fh:
            resp = requests.post(
                f"{API}/releases/{rid}/attach_files",
                data={"access_token": token},
                files={"file": ("SHA256SUMS.txt", fh)},
                timeout=120,
            )
        log("上传 SHA256SUMS.txt:", resp.status_code)

    log(f"完成：https://gitee.com/{OWNER}/{REPO}/releases/tag/v{version}")
    log("别忘了更新 release/version.json 里的 version / notes / downloadUrl")


if __name__ == "__main__":
    main()
