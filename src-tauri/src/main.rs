// 题鸣 Tauri 壳层入口
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    quizbox_lib::run()
}
