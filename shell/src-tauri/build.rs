fn main() {
    // The app manifest (Common Controls v6, the same one tauri-build embeds
    // by default) is linked into every target here instead of by
    // tauri-build, which puts it on the app binary alone: the unit tests are
    // the lib's, and without it their executable fails to load
    // (STATUS_ENTRYPOINT_NOT_FOUND, TaskDialogIndirect is comctl32 v6's).
    let windows = tauri_build::WindowsAttributes::new_without_app_manifest();
    tauri_build::try_build(tauri_build::Attributes::new().windows_attributes(windows))
        .expect("tauri-build");
    app_manifest();
}

#[cfg(windows)]
fn app_manifest() {
    use std::path::PathBuf;

    let dir = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR"));
    let manifest = dir.join("common-controls.manifest");
    println!("cargo:rerun-if-changed={}", manifest.display());

    if std::env::var("CARGO_CFG_TARGET_ENV").as_deref() == Ok("msvc") {
        println!("cargo:rustc-link-arg=/MANIFEST:EMBED");
        println!("cargo:rustc-link-arg=/MANIFESTINPUT:{}", manifest.display());
        return;
    }

    // GNU: a resource object made by windres (RT_MANIFEST, id 1)
    let out = PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR"));
    let rc = out.join("app-manifest.rc");
    let obj = out.join("app-manifest.o");
    let path = manifest.display().to_string().replace('\\', "/");
    std::fs::write(&rc, format!("1 24 \"{path}\"\n")).expect("write app-manifest.rc");
    let status = std::process::Command::new("windres")
        .arg(&rc)
        .arg("-O")
        .arg("coff")
        .arg("-o")
        .arg(&obj)
        .status()
        .expect("run windres (MinGW binutils on PATH)");
    assert!(status.success(), "windres failed on {}", rc.display());
    println!("cargo:rustc-link-arg={}", obj.display());
}

#[cfg(not(windows))]
fn app_manifest() {}
