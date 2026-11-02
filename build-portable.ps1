# 打 Windows 免安装版（绿色版）：把 release 产物收进一个文件夹再压成 zip。
# 不重新编译，先跑 .\build.ps1 出产物再跑本脚本。
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$conf = Get-Content .\src-tauri\tauri.conf.json -Raw | ConvertFrom-Json
$ver = $conf.version

$app = "src-tauri\target\release\quizbox-desktop.exe"
$eng = "src-tauri\target\release\engine.exe"
if (-not (Test-Path $app) -or -not (Test-Path $eng)) {
    throw "缺少 release 产物，先跑 .\build.ps1"
}

$stage = "dist-windows\Qecho-$ver-x64-portable"
if (Test-Path "dist-windows") { Remove-Item -Recurse -Force "dist-windows" }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

# 主程序改名成 Qecho.exe，与安装版一致；sidecar 必须相邻且叫 engine.exe，别改
Copy-Item $app (Join-Path $stage "Qecho.exe")
Copy-Item $eng (Join-Path $stage "engine.exe")
Copy-Item "src-tauri\icons\icon.ico" (Join-Path $stage "Qecho.ico")

$dataDir = Join-Path $env:APPDATA "com.quizbox.desktop"
@"
Qecho（题鸣）免安装版 $ver

用法
  1. 整个文件夹解压到任意位置（路径别带特殊字符即可）；
  2. 双击 Qecho.exe 运行，不需要管理员权限，也不写注册表；
  3. engine.exe 必须和 Qecho.exe 放在同一层目录，删掉就起不来。

数据位置
  数据不放在这个文件夹，仍在用户目录下的 AppData\Roaming\com.quizbox.desktop
  （本机实测路径：$dataDir），题库、题图、备份都在里面。
  卸载：删掉这个文件夹，再删掉上面那个数据目录即可，不留残留。

运行环境
  Windows 10 1803 及以上 / Windows 11，需要系统自带 WebView2 运行时（Win11 与较新的 Win10 默认已装）。
  如果启动后白屏或闪退，装一次 Microsoft Edge WebView2 Evergreen 运行时即可。

和安装版的区别
  安装版会装到 Program Files、建开始菜单与桌面快捷方式、注册卸载项；免安装版什么都没有，换机器直接拷文件夹。
"@ | Set-Content -Path (Join-Path $stage "使用说明.txt") -Encoding UTF8

$zip = "dist-windows\Qecho-$ver-x64-portable.zip"
Compress-Archive -Path "$stage\*" -DestinationPath $zip -CompressionLevel Optimal -Force

$item = Get-Item $zip
Write-Host ("portable: {0}  {1:N1} MB" -f $zip, ($item.Length / 1MB))
