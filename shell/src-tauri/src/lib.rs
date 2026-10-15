mod auto_launch;
mod settings;
mod sidecar;
mod tray;

use std::sync::Mutex;

use tauri::webview::PageLoadEvent;
use tauri::{Manager, RunEvent};
use tauri_plugin_log::{Target, TargetKind};

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let mut builder = tauri::Builder::default();

    #[cfg(windows)]
    {
        builder = builder.plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            log::info!("second instance launched; focusing existing window");
            tray::show_main(app);
        }));
    }

    let builder = builder
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
                let _ = window.set_skip_taskbar(true);
                log::info!("close requested: hidden to tray");
            }
        })
        .plugin(
            tauri_plugin_window_state::Builder::default()
                .with_state_flags(sidecar::window_state_flags())
                .build(),
        )
        .on_page_load(|webview, payload| {
            if webview.label() == "main" && payload.event() == PageLoadEvent::Finished {
                sidecar::apply_pending_error(webview.app_handle(), payload.url());
            }
        })
        .setup(|app| {
            let log_dir = settings::log_dir();
            if let Err(err) = std::fs::create_dir_all(&log_dir) {
                eprintln!("create log dir: {err}");
            }
            app.handle().plugin(
                tauri_plugin_log::Builder::default()
                    .level(log::LevelFilter::Info)
                    .targets([
                        Target::new(TargetKind::Stdout),
                        Target::new(TargetKind::Folder {
                            path: log_dir,
                            file_name: Some("shell".into()),
                        }),
                    ])
                    .max_file_size(5 * 1024 * 1024)
                    .build(),
            )?;

            let loaded = settings::load()?;
            app.manage(Mutex::new(sidecar::Engine::new(loaded.web_key)));
            tray::setup(app.handle())?;

            let handle = app.handle().clone();
            std::thread::spawn(move || {
                if let Err(err) = sidecar::boot(&handle) {
                    sidecar::show_error(&handle, &err);
                }
            });

            log::info!("AgentDock v{} started", env!("CARGO_PKG_VERSION"));
            Ok(())
        });

    let app = builder
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    app.run(|app_handle, event| {
        if let RunEvent::ExitRequested { api, code, .. } = &event {
            if code.is_none() {
                api.prevent_exit();
                return;
            }
            api.prevent_exit();
            sidecar::shutdown(app_handle);
            sidecar::save_window_state(app_handle);
            if let Some(tray) = app_handle.tray_by_id(tray::TRAY_ID) {
                let _ = tray.set_visible(false);
            }
            std::process::exit(0);
        }
    });
}
