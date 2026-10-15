use anyhow::Result;
use tauri::menu::{CheckMenuItemBuilder, MenuBuilder, MenuItemBuilder};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Manager};

use crate::auto_launch;
use crate::sidecar;

pub const TRAY_ID: &str = "agentdock";

pub fn setup(app: &AppHandle) -> Result<()> {
    let menu = build_menu(app)?;
    let mut tray = TrayIconBuilder::with_id(TRAY_ID)
        .tooltip("AgentDock")
        .menu(&menu)
        .show_menu_on_left_click(false)
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                show_main(tray.app_handle());
            }
        })
        .on_menu_event(|app, event| match event.id().as_ref() {
            "show" => show_main(app),
            "autostart" => toggle_autostart(app),
            "restart-engine" => restart_engine(app),
            "quit" => quit(app),
            other => log::debug!("unhandled tray menu id {other}"),
        });

    if let Some(icon) = app.default_window_icon() {
        tray = tray.icon(icon.clone());
    }

    tray.build(app)?;
    Ok(())
}

pub fn show_main(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.set_skip_taskbar(false);
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

fn toggle_autostart(app: &AppHandle) {
    if let Err(err) = auto_launch::toggle() {
        log::error!("toggle auto-launch: {err:#}");
    }
    refresh_menu(app);
}

fn restart_engine(app: &AppHandle) {
    let handle = app.clone();
    std::thread::spawn(move || {
        if let Err(err) = sidecar::restart(&handle) {
            sidecar::show_error(&handle, &err);
        }
    });
}

fn quit(app: &AppHandle) {
    sidecar::shutdown(app);
    sidecar::save_window_state(app);
    hide_tray(app);
    app.exit(0);
}

fn hide_tray(app: &AppHandle) {
    if let Some(tray) = app.tray_by_id(TRAY_ID) {
        let _ = tray.set_visible(false);
    }
}

fn refresh_menu(app: &AppHandle) {
    match build_menu(app) {
        Ok(menu) => {
            if let Some(tray) = app.tray_by_id(TRAY_ID) {
                if let Err(err) = tray.set_menu(Some(menu)) {
                    log::error!("refresh tray menu: {err}");
                }
            }
        }
        Err(err) => log::error!("build tray menu: {err:#}"),
    }
}

fn build_menu(app: &AppHandle) -> Result<tauri::menu::Menu<tauri::Wry>> {
    let show = MenuItemBuilder::with_id("show", "显示主窗口 / Show").build(app)?;
    let autostart = CheckMenuItemBuilder::with_id("autostart", "开机自启")
        .checked(auto_launch::is_enabled().unwrap_or(false))
        .build(app)?;
    let restart =
        MenuItemBuilder::with_id("restart-engine", "重启引擎 / Restart engine").build(app)?;
    let quit = MenuItemBuilder::with_id("quit", "退出 / Quit").build(app)?;
    Ok(MenuBuilder::new(app)
        .item(&show)
        .item(&autostart)
        .separator()
        .item(&restart)
        .separator()
        .item(&quit)
        .build()?)
}
