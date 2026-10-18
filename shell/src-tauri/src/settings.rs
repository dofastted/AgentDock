use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use rand::Rng;
use serde::{Deserialize, Serialize};

pub const DEFAULT_UI_PORT: u16 = 3430;
const KEY_LEN: usize = 32;
const KEY_CHARS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ShellSettings {
    #[serde(rename = "webKey")]
    pub web_key: String,
    #[serde(rename = "uiPort")]
    pub ui_port: u16,
}

pub fn app_dir() -> PathBuf {
    // Check if running in portable mode (.portable marker in exe directory)
    if let Ok(exe) = std::env::current_exe() {
        if let Some(exe_dir) = exe.parent() {
            let portable_marker = exe_dir.join(".portable");
            if portable_marker.exists() {
                return exe_dir.join("data");
            }
        }
    }
    
    // Standard installation: use AppData
    if let Ok(appdata) = std::env::var("APPDATA") {
        return PathBuf::from(appdata).join("AgentDock");
    }
    PathBuf::from(".").join("AgentDock")
}

pub fn settings_path() -> PathBuf {
    app_dir().join("shell.json")
}

pub fn log_dir() -> PathBuf {
    app_dir().join("logs")
}

pub fn engine_log_path() -> PathBuf {
    log_dir().join("engine.log")
}

pub fn load() -> Result<ShellSettings> {
    let path = settings_path();
    if path.exists() {
        let raw = fs::read_to_string(&path).with_context(|| format!("read {}", path.display()))?;
        if let Ok(parsed) = serde_json::from_str::<ShellSettings>(&raw) {
            if valid_key(&parsed.web_key) {
                let ui_port = if parsed.ui_port == 0 {
                    DEFAULT_UI_PORT
                } else {
                    parsed.ui_port
                };
                return Ok(ShellSettings {
                    web_key: parsed.web_key,
                    ui_port,
                });
            }
        }
        log::warn!("shell.json missing or invalid; generating a new web key");
    }

    let settings = ShellSettings {
        web_key: generate_web_key(),
        ui_port: DEFAULT_UI_PORT,
    };
    save(&settings)?;
    Ok(settings)
}

pub fn save(settings: &ShellSettings) -> Result<()> {
    let dir = app_dir();
    fs::create_dir_all(&dir).with_context(|| format!("create {}", dir.display()))?;
    let path = settings_path();
    let body = serde_json::to_vec_pretty(settings).context("serialize shell.json")?;
    fs::write(&path, body).with_context(|| format!("write {}", path.display()))?;
    Ok(())
}

pub fn generate_web_key() -> String {
    let mut rng = rand::thread_rng();
    (0..KEY_LEN)
        .map(|_| KEY_CHARS[rng.gen_range(0..KEY_CHARS.len())] as char)
        .collect()
}

pub fn valid_key(key: &str) -> bool {
    key.len() == KEY_LEN && key.bytes().all(|b| KEY_CHARS.contains(&b))
}

pub fn engine_log_display() -> String {
    display_path(&engine_log_path())
}

fn display_path(path: &Path) -> String {
    path.to_string_lossy().into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generated_key_matches_contract() {
        let key = generate_web_key();
        assert!(valid_key(&key), "{key}");
        assert_eq!(key.len(), 32);
    }

    #[test]
    fn rejects_short_or_punctuated_keys() {
        assert!(!valid_key("short"));
        assert!(!valid_key("0123456789abcdef0123456789abcd!"));
        assert!(!valid_key("0123456789abcdef0123456789abcde"));
    }
}
