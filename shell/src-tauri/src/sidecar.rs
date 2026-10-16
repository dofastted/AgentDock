use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use anyhow::{anyhow, bail, Context, Result};
use tauri::{AppHandle, Manager, WebviewWindow};
use tauri_plugin_window_state::{AppHandleExt, StateFlags};

use crate::settings;

const CREATE_NO_WINDOW: u32 = 0x08000000;
const READY_TIMEOUT: Duration = Duration::from_secs(20);
const READY_INTERVAL: Duration = Duration::from_millis(200);
const LOG_ROTATE_BYTES: u64 = 5 * 1024 * 1024;
const MAX_CRASH_RESTARTS: u8 = 3;

pub struct Engine {
    job: Option<Job>,
    child: Option<Child>,
    port: u16,
    key: String,
    splash_url: Option<tauri::Url>,
    crash_restarts: u8,
    stopping: bool,
    restarting: bool,
    ready: bool,
    pending_error: Option<SplashError>,
}

struct SplashError {
    message: String,
    log_path: String,
}

struct Job {
    handle: windows_sys::Win32::Foundation::HANDLE,
}

// Job handles are process-wide kernel objects; the manager is moved across threads.
unsafe impl Send for Job {}
unsafe impl Sync for Job {}

impl Drop for Job {
    fn drop(&mut self) {
        unsafe {
            let _ = windows_sys::Win32::Foundation::CloseHandle(self.handle);
        }
    }
}

impl Engine {
    pub fn new(key: String) -> Self {
        Self {
            job: None,
            child: None,
            port: settings::DEFAULT_UI_PORT,
            key,
            splash_url: None,
            crash_restarts: 0,
            stopping: false,
            restarting: false,
            ready: false,
            pending_error: None,
        }
    }
}

pub fn window_state_flags() -> StateFlags {
    StateFlags::POSITION | StateFlags::SIZE | StateFlags::MAXIMIZED
}

pub fn boot(app: &AppHandle) -> Result<()> {
    capture_splash(app);
    spawn_engine(app, true)?;
    wait_until_ready(app)?;
    navigate_to_ui(app)?;
    watch_child(app.clone());
    Ok(())
}

pub fn restart(app: &AppHandle) -> Result<()> {
    with_engine(app, |engine| {
        engine.restarting = true;
        engine.crash_restarts = 0;
        engine.ready = false;
        engine.pending_error = None;
        kill_child(engine);
        Ok(())
    })?;
    std::thread::sleep(Duration::from_millis(250));
    spawn_engine(app, false)?;
    wait_until_ready(app)?;
    navigate_to_ui(app)?;
    let _ = with_engine(app, |engine| {
        engine.restarting = false;
        Ok(())
    });
    Ok(())
}

pub fn shutdown(app: &AppHandle) {
    let _ = with_engine(app, |engine| {
        engine.stopping = true;
        engine.ready = false;
        kill_child(engine);
        engine.job = None;
        Ok(())
    });
}

pub fn apply_pending_error(app: &AppHandle, url: &tauri::Url) {
    if url.host_str() == Some("127.0.0.1") {
        return;
    }
    let Some(state) = app.try_state::<Mutex<Engine>>() else {
        return;
    };
    let Ok(engine) = state.lock() else {
        return;
    };
    let Some(err) = engine.pending_error.as_ref() else {
        return;
    };
    let message = err.message.clone();
    let log_path = err.log_path.clone();
    drop(engine);
    if let Some(window) = app.get_webview_window("main") {
        push_status(&window, "error", &message, Some(&log_path));
    }
}

pub fn show_error(app: &AppHandle, err: &anyhow::Error) {
    let log_path = settings::engine_log_display();
    log::error!("engine error: {err:#}");
    let message = format!("{err:#}");
    let _ = with_engine(app, |engine| {
        engine.ready = false;
        engine.restarting = false;
        engine.pending_error = Some(SplashError {
            message: message.clone(),
            log_path: log_path.clone(),
        });
        Ok(())
    });
    restore_splash(app);
    if let Some(window) = app.get_webview_window("main") {
        push_status(&window, "error", &message, Some(&log_path));
    }
}

fn spawn_engine(app: &AppHandle, initial: bool) -> Result<()> {
    let sidecar = sidecar_path()?;
    let app_dir = settings::app_dir();
    let magpie = app_dir.join("magpie");
    fs::create_dir_all(&app_dir).with_context(|| format!("create {}", app_dir.display()))?;
    fs::create_dir_all(&magpie).with_context(|| format!("create {}", magpie.display()))?;
    fs::create_dir_all(settings::log_dir())
        .with_context(|| format!("create {}", settings::log_dir().display()))?;

    let preferred = settings::load()
        .map(|s| s.ui_port)
        .unwrap_or(settings::DEFAULT_UI_PORT);
    let port = pick_listen_port(preferred)?;
    let log_file = open_engine_log()?;
    let stderr = log_file.try_clone().context("clone engine log handle")?;

    with_engine(app, move |engine| {
        if engine.stopping {
            bail!("engine is shutting down");
        }
        if engine.job.is_none() {
            engine.job = Some(create_job()?);
        }
        let key = engine.key.clone();
        let job_handle = engine
            .job
            .as_ref()
            .map(|job| job.handle)
            .ok_or_else(|| anyhow!("job object missing"))?;

        let mut cmd = Command::new(&sidecar);
        cmd.arg("serve")
            .arg(format!("--web=127.0.0.1:{port}"))
            // Isolate engine state without redirecting external CLI config roots.
            .env("AGENTDOCK_DATA_DIR", &magpie)
            .env("MAGPIE_WEB_KEY", &key)
            .stdin(Stdio::null())
            .stdout(log_file)
            .stderr(stderr);
        apply_create_no_window(&mut cmd);

        let child = cmd
            .spawn()
            .with_context(|| format!("spawn {}", sidecar.display()))?;
        assign_to_job(job_handle, &child)?;

        log::info!(
            "spawned engine pid={} port={} initial={initial}",
            child.id(),
            port
        );
        engine.child = Some(child);
        engine.port = port;
        engine.ready = false;
        Ok(())
    })
}

fn wait_until_ready(app: &AppHandle) -> Result<()> {
    let port = with_engine(app, |engine| Ok(engine.port))?;
    let deadline = Instant::now() + READY_TIMEOUT;
    loop {
        if with_engine(app, |engine| Ok(engine.stopping))? {
            bail!("engine is shutting down");
        }
        if child_exited(app)? {
            let log_path = settings::engine_log_display();
            bail!("engine exited before becoming ready\n{log_path}");
        }
        if http_responded(port) {
            let _ = with_engine(app, |engine| {
                engine.ready = true;
                Ok(())
            });
            log::info!("engine ready on 127.0.0.1:{port}");
            return Ok(());
        }
        if Instant::now() >= deadline {
            let log_path = settings::engine_log_display();
            bail!("engine did not become ready within 20s\n{log_path}");
        }
        std::thread::sleep(READY_INTERVAL);
    }
}

fn navigate_to_ui(app: &AppHandle) -> Result<()> {
    let (port, key) = with_engine(app, |engine| Ok((engine.port, engine.key.clone())))?;
    let mut url =
        tauri::Url::parse(&format!("http://127.0.0.1:{port}/")).context("engine ui url")?;
    url.set_query(Some(&format!("k={key}")));
    let window = main_window(app)?;
    window.navigate(url).context("navigate to engine ui")?;
    Ok(())
}

fn watch_child(app: AppHandle) {
    std::thread::spawn(move || loop {
        std::thread::sleep(Duration::from_millis(400));
        let snapshot = match with_engine(&app, |engine| {
            if engine.stopping {
                return Ok(None);
            }
            Ok(Some((
                engine.restarting,
                engine.ready,
                engine.child.is_some(),
            )))
        }) {
            Ok(Some(snapshot)) => snapshot,
            Ok(None) => return,
            Err(_) => return,
        };
        if snapshot.0 || !snapshot.2 {
            continue;
        }
        let exited = match child_exited(&app) {
            Ok(exited) => exited,
            Err(_) => return,
        };
        if !exited {
            continue;
        }
        let was_ready = snapshot.1;
        if !was_ready {
            continue;
        }
        let n = match with_engine(&app, |engine| {
            if engine.stopping || engine.restarting {
                return Ok(None);
            }
            engine.ready = false;
            engine.crash_restarts = engine.crash_restarts.saturating_add(1);
            Ok(Some(engine.crash_restarts))
        }) {
            Ok(Some(n)) => n,
            Ok(None) => continue,
            Err(_) => return,
        };
        if n > MAX_CRASH_RESTARTS {
            show_error(
                &app,
                &anyhow!("engine kept exiting after {MAX_CRASH_RESTARTS} restarts"),
            );
            return;
        }
        let backoff = 1u64 << (n - 1).min(2);
        log::warn!("engine exited unexpectedly; restart {n} in {backoff}s");
        std::thread::sleep(Duration::from_secs(backoff));
        let skip = with_engine(&app, |e| Ok(e.stopping || e.restarting)).unwrap_or(true);
        if skip {
            continue;
        }
        match respawn_after_crash(&app) {
            Ok(()) => {}
            Err(err) => {
                show_error(&app, &err);
                return;
            }
        }
    });
}

fn respawn_after_crash(app: &AppHandle) -> Result<()> {
    spawn_engine(app, false)?;
    wait_until_ready(app)?;
    navigate_to_ui(app)?;
    Ok(())
}

fn child_exited(app: &AppHandle) -> Result<bool> {
    with_engine(app, |engine| {
        let Some(child) = engine.child.as_mut() else {
            return Ok(false);
        };
        match child.try_wait() {
            Ok(Some(_)) => {
                engine.child = None;
                Ok(true)
            }
            Ok(None) => Ok(false),
            Err(err) => {
                log::warn!("wait engine: {err}");
                Ok(false)
            }
        }
    })
}

fn kill_child(engine: &mut Engine) {
    if let Some(mut child) = engine.child.take() {
        let _ = child.kill();
        let _ = child.wait();
    }
}

fn capture_splash(app: &AppHandle) {
    let Ok(window) = main_window(app) else {
        return;
    };
    if let Ok(url) = window.url() {
        let _ = with_engine(app, |engine| {
            engine.splash_url = Some(url);
            Ok(())
        });
    }
}

fn restore_splash(app: &AppHandle) {
    let splash = with_engine(app, |engine| Ok(engine.splash_url.clone()))
        .ok()
        .flatten();
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    if is_engine_url(&window) {
        if let Some(url) = splash {
            let _ = window.navigate(url);
        }
    }
}

fn push_status(window: &WebviewWindow, state: &str, message: &str, log_path: Option<&str>) {
    let payload = serde_json::json!({
        "state": state,
        "message": message,
        "logPath": log_path,
    });
    let js = format!("window.__agentdockSetStatus && window.__agentdockSetStatus({payload});");
    if let Err(err) = window.eval(&js) {
        log::warn!("push splash status: {err}");
    }
}

fn is_engine_url(window: &WebviewWindow) -> bool {
    window
        .url()
        .ok()
        .and_then(|url| url.host_str().map(str::to_string))
        .as_deref()
        == Some("127.0.0.1")
}

fn main_window(app: &AppHandle) -> Result<WebviewWindow> {
    app.get_webview_window("main")
        .ok_or_else(|| anyhow!("main window missing"))
}

fn with_engine<T>(app: &AppHandle, f: impl FnOnce(&mut Engine) -> Result<T>) -> Result<T> {
    let state = app.state::<Mutex<Engine>>();
    let mut engine = state.lock().map_err(|_| anyhow!("engine state poisoned"))?;
    f(&mut engine)
}

fn sidecar_path() -> Result<PathBuf> {
    let exe = std::env::current_exe().context("current executable path")?;
    let beside = exe.with_file_name("agentdock-engine.exe");
    if beside.is_file() {
        return Ok(beside);
    }
    let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("binaries")
        .join("agentdock-engine-x86_64-pc-windows-msvc.exe");
    if dev.is_file() {
        return Ok(dev);
    }
    bail!(
        "engine sidecar not found at {} or {}",
        beside.display(),
        dev.display()
    );
}

fn pick_listen_port(preferred: u16) -> Result<u16> {
    match TcpListener::bind(("127.0.0.1", preferred)) {
        Ok(listener) => {
            let port = listener.local_addr()?.port();
            drop(listener);
            Ok(port)
        }
        Err(_) => {
            let listener = TcpListener::bind(("127.0.0.1", 0)).context("bind ephemeral port")?;
            let port = listener.local_addr()?.port();
            drop(listener);
            Ok(port)
        }
    }
}

fn http_responded(port: u16) -> bool {
    let addr = match format!("127.0.0.1:{port}").parse() {
        Ok(addr) => addr,
        Err(_) => return false,
    };
    let Ok(mut stream) = TcpStream::connect_timeout(&addr, Duration::from_millis(150)) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_millis(200)));
    let req = format!("GET / HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\nConnection: close\r\n\r\n");
    if stream.write_all(req.as_bytes()).is_err() {
        return false;
    }
    let mut buf = [0u8; 16];
    matches!(stream.read(&mut buf), Ok(n) if n >= 5 && buf.starts_with(b"HTTP/"))
}

fn open_engine_log() -> Result<File> {
    let path = settings::engine_log_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    rotate_if_needed(&path)?;
    OpenOptions::new()
        .create(true)
        .append(true)
        .open(&path)
        .with_context(|| format!("open {}", path.display()))
}

fn rotate_if_needed(path: &Path) -> Result<()> {
    let Ok(meta) = fs::metadata(path) else {
        return Ok(());
    };
    if meta.len() < LOG_ROTATE_BYTES {
        return Ok(());
    }
    let rotated = path.with_file_name("engine.log.1");
    let _ = fs::remove_file(&rotated);
    fs::rename(path, &rotated).with_context(|| format!("rotate {}", path.display()))?;
    Ok(())
}

fn apply_create_no_window(cmd: &mut Command) {
    use std::os::windows::process::CommandExt;
    cmd.creation_flags(CREATE_NO_WINDOW);
}

fn create_job() -> Result<Job> {
    use windows_sys::Win32::Foundation::{GetLastError, HANDLE};
    use windows_sys::Win32::System::JobObjects::{
        CreateJobObjectW, JobObjectExtendedLimitInformation, SetInformationJobObject,
        JOBOBJECT_EXTENDED_LIMIT_INFORMATION, JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
    };

    unsafe {
        let handle: HANDLE = CreateJobObjectW(std::ptr::null(), std::ptr::null());
        if handle.is_null() {
            bail!("CreateJobObjectW failed ({})", GetLastError());
        }
        let mut info = JOBOBJECT_EXTENDED_LIMIT_INFORMATION::default();
        info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
        let ok = SetInformationJobObject(
            handle,
            JobObjectExtendedLimitInformation,
            std::ptr::from_ref(&info).cast(),
            std::mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
        );
        if ok == 0 {
            let err = GetLastError();
            let _ = windows_sys::Win32::Foundation::CloseHandle(handle);
            bail!("SetInformationJobObject failed ({err})");
        }
        Ok(Job { handle })
    }
}

fn assign_to_job(job: windows_sys::Win32::Foundation::HANDLE, child: &Child) -> Result<()> {
    use std::os::windows::io::AsRawHandle;
    use windows_sys::Win32::Foundation::GetLastError;
    use windows_sys::Win32::System::JobObjects::AssignProcessToJobObject;

    unsafe {
        let process = child.as_raw_handle() as windows_sys::Win32::Foundation::HANDLE;
        let ok = AssignProcessToJobObject(job, process);
        if ok == 0 {
            bail!("AssignProcessToJobObject failed ({})", GetLastError());
        }
    }
    Ok(())
}

pub fn save_window_state(app: &AppHandle) {
    if let Err(err) = app.save_window_state(window_state_flags()) {
        log::error!("save window state: {err}");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn preferred_port_is_returned_when_free() {
        let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        drop(listener);
        let got = pick_listen_port(port).unwrap();
        assert_eq!(got, port);
    }
}
