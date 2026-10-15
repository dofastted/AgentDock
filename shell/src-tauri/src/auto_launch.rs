use anyhow::{Context, Result};
use auto_launch::AutoLaunchBuilder;

fn instance() -> Result<auto_launch::AutoLaunch> {
    let exe = std::env::current_exe().context("current executable path")?;
    AutoLaunchBuilder::new()
        .set_app_name("AgentDock")
        .set_app_path(&exe.to_string_lossy())
        .build()
        .context("create AutoLaunch")
}

pub fn enable() -> Result<()> {
    instance()?.enable().context("enable auto-launch")?;
    log::info!("auto-launch enabled");
    Ok(())
}

pub fn disable() -> Result<()> {
    instance()?.disable().context("disable auto-launch")?;
    log::info!("auto-launch disabled");
    Ok(())
}

pub fn is_enabled() -> Result<bool> {
    instance()?.is_enabled().context("query auto-launch")
}

pub fn toggle() -> Result<bool> {
    if is_enabled()? {
        disable()?;
        Ok(false)
    } else {
        enable()?;
        Ok(true)
    }
}
