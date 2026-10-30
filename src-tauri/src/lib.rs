// 题鸣 Tauri 壳层：拉起 Go 引擎 sidecar、下发地址、托盘与优雅关闭。
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::io::Write;
use std::net::{SocketAddr, TcpStream};
use std::sync::mpsc;
use std::sync::Mutex;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use tauri::{Manager, State};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

struct CloseBehavior(Mutex<String>);

struct EngineState {
    base_url: Mutex<String>,
    port: Mutex<Option<u16>>,
    shutdown_token: String,
    child: Mutex<Option<CommandChild>>,
}

#[tauri::command]
fn engine_base_url(state: State<'_, EngineState>) -> String {
    state.base_url.lock().unwrap().clone()
}

#[tauri::command]
fn request_lan_access() -> Result<(), String> {
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let dir = exe.parent().ok_or("无法定位程序目录")?;
    let mut programs = vec![exe.clone()];
    let engine = dir.join("engine.exe");
    let dev_engine = dir.join("binaries").join("engine-x86_64-pc-windows-msvc.exe");
    if engine.exists() {
        programs.push(engine);
    } else if dev_engine.exists() {
        programs.push(dev_engine);
    }

    // 带 BOM 写出，Windows PowerShell 5.1 才会按 UTF-8 读取规则名；顺带清理旧品牌规则
    let mut script = String::from("\u{feff}$ErrorActionPreference = 'SilentlyContinue'\n");
    script.push_str("Remove-NetFirewallRule -DisplayName '题鸣 LAN' | Out-Null\n");
    script.push_str("Remove-NetFirewallRule -DisplayName 'QuizBox LAN' | Out-Null\n");
    script.push_str("Remove-NetFirewallRule -DisplayName 'Qecho LAN' | Out-Null\n");
    for program in programs {
        let path = program.to_string_lossy().replace('\'', "''");
        script.push_str(&format!(
            "New-NetFirewallRule -DisplayName 'Qecho LAN' -Direction Inbound -Program '{path}' -Action Allow -Profile Any | Out-Null\n"
        ));
    }
    let script_path = std::env::temp_dir().join("timing-lan-firewall.ps1");
    std::fs::write(&script_path, script).map_err(|e| e.to_string())?;
    let script_arg = script_path.to_string_lossy().replace('\'', "''");
    let command = format!(
        "Start-Process powershell -Verb RunAs -WindowStyle Hidden -Wait -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','{script_arg}'"
    );
    std::process::Command::new("powershell")
        .args(["-NoProfile", "-Command", &command])
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn set_window_size(app: tauri::AppHandle, width: f64, height: f64, resizable: Option<bool>) {
    let resizable = resizable.unwrap_or(true);
    if let Some(win) = app.get_webview_window("main") {
        let monitor = win
            .current_monitor()
            .ok()
            .flatten()
            .or_else(|| win.primary_monitor().ok().flatten());
        if let Some(monitor) = monitor {
            let scale = monitor.scale_factor();
            let screen = monitor.size();
            let avail_w = (screen.width as f64 - 48.0).max(800.0);
            let avail_h = (screen.height as f64 - 96.0).max(560.0);
            let w = (width * scale).min(avail_w);
            let h = (height * scale).min(avail_h);
            if resizable {
                let min_w = (780.0 * scale).min(w);
                let min_h = (460.0 * scale).min(h);
                let _ = win.set_min_size(Some(tauri::PhysicalSize::new(min_w, min_h)));
            } else {
                let _ = win.set_min_size(None::<tauri::PhysicalSize<f64>>);
            }
            let _ = win.set_size(tauri::PhysicalSize::new(w, h));
        } else {
            let _ = win.set_size(tauri::LogicalSize::new(width, height));
            if resizable {
                let _ = win.set_min_size(Some(tauri::LogicalSize::new(
                    width.min(780.0),
                    height.min(460.0),
                )));
            } else {
                let _ = win.set_min_size(None::<tauri::LogicalSize<f64>>);
            }
        }
        let _ = win.set_resizable(resizable);
        let _ = win.center();
        let _ = win.show();
        let _ = win.set_focus();
    }
}

/// 弹出系统保存对话框，返回用户选择的保存路径（取消时返回 null）。
#[tauri::command]
async fn pick_export_path(app: tauri::AppHandle, file_name: String) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;
    let picked = app
        .dialog()
        .file()
        .set_title("导出题库")
        .set_file_name(&file_name)
        .add_filter("Excel 工作簿", &["xlsx"])
        .blocking_save_file();
    match picked {
        None => Ok(None),
        Some(path) => {
            let path = path.into_path().map_err(|e| e.to_string())?;
            Ok(Some(path.to_string_lossy().into_owned()))
        }
    }
}

#[tauri::command]
fn set_close_behavior(state: State<'_, CloseBehavior>, behavior: String) {
    if behavior == "exit" || behavior == "minimize" {
        *state.0.lock().unwrap() = behavior;
    }
}

fn show_main(app: &tauri::AppHandle) {
    if let Some(win) = app.get_webview_window("main") {
        let _ = win.show();
        let _ = win.unminimize();
        let _ = win.set_focus();
    }
}

fn start_engine(
    app: &tauri::AppHandle,
    data_dir: &str,
    shutdown_token: &str,
) -> (Option<u16>, Option<CommandChild>) {
    let command = match app.shell().sidecar("engine") {
        Ok(cmd) => cmd,
        Err(err) => {
            eprintln!("[shell] locate engine sidecar failed: {err}");
            return (None, None);
        }
    };
    // 开发模式下固定 54310，浏览器访问 vite(54311) 时 /api 代理才能命中同一个引擎；发布版仍动态分配。
    let port = if cfg!(debug_assertions) { "54310" } else { "0" };
    let command = command
        .args([
            "--host",
            "0.0.0.0",
            "--port",
            port,
            "--data-dir",
            data_dir,
            "--desktop",
        ])
        .env("QUIZBOX_SHUTDOWN_TOKEN", shutdown_token);

    let (mut rx, child) = match command.spawn() {
        Ok(pair) => pair,
        Err(err) => {
            eprintln!("[shell] spawn engine failed: {err}");
            return (None, None);
        }
    };

    let (port_tx, port_rx) = mpsc::channel::<u16>();
    tauri::async_runtime::spawn(async move {
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(bytes) => {
                    let line = String::from_utf8_lossy(&bytes);
                    let line = line.trim();
                    if let Some(value) = line.strip_prefix("ENGINE_PORT=") {
                        if let Ok(port) = value.parse::<u16>() {
                            let _ = port_tx.send(port);
                        }
                    }
                    println!("[engine] {line}");
                }
                CommandEvent::Stderr(bytes) => {
                    eprintln!("[engine:err] {}", String::from_utf8_lossy(&bytes).trim());
                }
                CommandEvent::Terminated(payload) => {
                    eprintln!("[engine] terminated: code={:?}", payload.code);
                    break;
                }
                _ => {}
            }
        }
    });

    let port = port_rx.recv_timeout(Duration::from_secs(10)).ok();
    match port {
        Some(p) => println!("[shell] engine ready on 127.0.0.1:{p}"),
        None => eprintln!("[shell] engine port not received within 10s"),
    }
    (port, Some(child))
}

fn shutdown_engine(app: &tauri::AppHandle) {
    let state = app.state::<EngineState>();
    let port = *state.port.lock().unwrap();
    if let Some(port) = port {
        let addr = SocketAddr::from(([127, 0, 0, 1], port));
        let request = format!(
            "POST /api/v1/system/shutdown HTTP/1.1\r\nHost: 127.0.0.1\r\nX-Shutdown-Token: {}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
            state.shutdown_token
        );
        if let Ok(mut stream) = TcpStream::connect_timeout(&addr, Duration::from_millis(500)) {
            let _ = stream.set_write_timeout(Some(Duration::from_millis(300)));
            let _ = stream.write_all(request.as_bytes());
        }
        for _ in 0..30 {
            if TcpStream::connect_timeout(&addr, Duration::from_millis(100)).is_err() {
                break;
            }
            std::thread::sleep(Duration::from_millis(100));
        }
    }
    let child = state.child.lock().unwrap().take();
    if let Some(child) = child {
        let _ = child.kill();
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            show_main(app);
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .manage(CloseBehavior(Mutex::new("minimize".to_string())))
        .setup(|app| {
            let app_data = app.path().app_data_dir()?;
            std::fs::create_dir_all(&app_data)?;
            let data_dir = app_data.to_string_lossy().to_string();
            let shutdown_token = format!(
                "{:x}{:x}",
                std::process::id(),
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .map(|d| d.as_nanos())
                    .unwrap_or_default()
            );
            let (port, child) = start_engine(app.handle(), &data_dir, &shutdown_token);
            let base_url = match port {
                Some(p) => format!("http://127.0.0.1:{p}/api/v1"),
                None => "http://127.0.0.1:54310/api/v1".to_string(),
            };
            app.manage(EngineState {
                base_url: Mutex::new(base_url),
                port: Mutex::new(port),
                shutdown_token,
                child: Mutex::new(child),
            });

            let show = tauri::menu::MenuItem::with_id(app, "show", "打开主界面", true, None::<&str>)?;
            let quit = tauri::menu::MenuItem::with_id(app, "quit", "退出程序", true, None::<&str>)?;
            let menu = tauri::menu::Menu::with_items(app, &[&show, &quit])?;
            tauri::tray::TrayIconBuilder::with_id("main-tray")
                .icon(app.default_window_icon().expect("missing app icon").clone())
                .tooltip("Qecho")
                .menu(&menu)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "show" => show_main(app),
                    "quit" => app.exit(0),
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let tauri::tray::TrayIconEvent::Click {
                        button: tauri::tray::MouseButton::Left,
                        button_state: tauri::tray::MouseButtonState::Up,
                        ..
                    } = event
                    {
                        show_main(tray.app_handle());
                    }
                })
                .build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                let behavior = window
                    .app_handle()
                    .state::<CloseBehavior>()
                    .0
                    .lock()
                    .unwrap()
                    .clone();
                if behavior == "minimize" {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            engine_base_url,
            set_window_size,
            set_close_behavior,
            request_lan_access,
            pick_export_path
        ])
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    app.run(|app_handle, event| {
        if let tauri::RunEvent::Exit = event {
            shutdown_engine(app_handle);
        }
    });
}
