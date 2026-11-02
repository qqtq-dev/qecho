#!/usr/bin/env python3
"""改写 tar.gz 内指定成员的权限位（mode）。

背景
----
在 Windows/MSYS 上给「没有扩展名」的 ELF 二进制设可执行位会静默失效：
chmod 755 返回 0，stat 仍然是 644，install -m 755 也救不回来。所以
「先让磁盘上的 quizbox-engine 变成 755 再打 tar」这条路在 Windows 上不成立，
打出来的包解压后引擎是 644，用户 ./quizbox-engine 直接 Permission denied。

本脚本不碰文件系统，直接改写归档内部的 mode 字段，因此与打包平台无关。
写完后会回读校验，任何一条规则没生效都以非 0 退出，避免「改了但没生效」
这种静默失败再次混进发行版。

用法
----
    python scripts/tar-mode.py 包.tar.gz 0755=quizbox-web-0.2.1/quizbox-engine

规则格式 MODE=PATH：
    MODE  八进制权限，755 或 0755 都行
    PATH  归档内的成员名；以 / 结尾表示这个目录下的全部内容（含目录本身）

多条规则按顺序匹配，后面的覆盖前面的。
"""

import gzip
import os
import sys
import tarfile


USAGE = "用法：tar-mode.py 包.tar.gz 0755=dir/quizbox-engine [0755=dir/install.sh ...]"


def die(msg: str):
    print("tar-mode: " + msg, file=sys.stderr)
    sys.exit(1)


def parse_rule(text: str):
    if "=" not in text:
        die("规则要写成 MODE=PATH，收到：%s" % text)
    mode_s, path = text.split("=", 1)
    try:
        mode = int(mode_s, 8)
    except ValueError:
        die("权限位不是八进制：%s" % mode_s)
    if not 0 <= mode <= 0o7777:
        die("权限位超出范围：%s" % mode_s)
    if not path:
        die("规则缺少路径：%s" % text)
    return mode, path


def hit(name: str, path: str) -> bool:
    if path.endswith("/"):
        base = path.rstrip("/")
        return name == base or name.startswith(base + "/")
    return name == path


def main(argv):
    if len(argv) < 3:
        print(USAGE, file=sys.stderr)
        return 2

    archive = argv[1]
    if not os.path.isfile(archive):
        die("归档不存在：%s" % archive)
    rules = [parse_rule(a) for a in argv[2:]]

    matched = [False] * len(rules)   # 规则是否命中过（含被跳过的链接）
    changed = [[] for _ in rules]    # 真正改过 mode 的成员名

    tmp = archive + ".tmp"
    try:
        with tarfile.open(archive, "r:gz") as src, open(tmp, "wb") as raw:
            # mtime=0：gzip 头不写时间戳，同样的输入产出同样的包
            gz = gzip.GzipFile(filename="", mode="wb", fileobj=raw,
                               compresslevel=9, mtime=0)
            try:
                with tarfile.open(fileobj=gz, mode="w|",
                                  format=tarfile.GNU_FORMAT) as dst:
                    for info in src:
                        for i, (mode, path) in enumerate(rules):
                            if not hit(info.name, path):
                                continue
                            matched[i] = True
                            # 符号链接 / 硬链接没有独立权限位，改了也没意义
                            if info.issym() or info.islnk():
                                continue
                            if info.mode != mode:
                                info.mode = mode
                                changed[i].append(info.name)
                            break
                        if info.isreg():
                            dst.addfile(info, src.extractfile(info))
                        else:
                            dst.addfile(info)
            finally:
                gz.close()
    except Exception as exc:  # noqa: BLE001 打包失败要原样报出来
        if os.path.exists(tmp):
            os.remove(tmp)
        die("改写失败：%s" % exc)

    # 先校验临时文件，通过了才覆盖原包——校验失败时原包一个字节都没动
    errors = []
    for i, (mode, path) in enumerate(rules):
        if not matched[i]:
            errors.append("规则没命中任何成员：%s" % path)

    # 回读校验：命中项的权限位必须真的是设定值
    if not errors:
        with tarfile.open(tmp, "r:gz") as t:
            modes = {m.name: m.mode for m in t}
        for i, (mode, _path) in enumerate(rules):
            for name in changed[i]:
                actual = modes.get(name)
                if actual is None:
                    errors.append("%s 改完后在归档里找不到了" % name)
                elif actual & 0o777 != mode & 0o777:
                    errors.append("%s 期望 %03o，实际 %03o"
                                  % (name, mode & 0o777, actual & 0o777))

    if errors:
        os.remove(tmp)
        for e in errors:
            print("tar-mode: " + e, file=sys.stderr)
        die("校验未通过，原包未改动")

    os.replace(tmp, archive)

    print("== 归档权限已修正（%s）" % os.path.basename(archive))
    for i, (mode, _path) in enumerate(rules):
        for name in changed[i]:
            print("   %03o  %s" % (mode & 0o777, name))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
