use anyhow::Result;
use std::path::Path;
use notify::{Watcher, RecursiveMode, RecommendedWatcher, Config};
use std::sync::mpsc::channel;
use std::time::Duration;
use crate::core::config::load_config;
use crate::core::api::ApiClient;
use crate::watcher::scanner::Scanner;

pub async fn run(path: &str, org_id: Option<String>, full_index: bool) -> Result<()> {
    let config = load_config()?;
    let client = ApiClient::new(&config.backend_url);
    let oid = org_id.unwrap_or("public".to_string());
    
    let scanner = Scanner::new(ApiClient::new(&config.backend_url), oid.clone()); // Clone for scanner
    
    // Initial Scan
    if full_index {
        scanner.scan(Path::new(path)).await;
    }

    // Watcher Setup
    println!("Starting watcher on: {}", path);
    
    let (tx, rx) = channel();
    
    // Automatically select the best implementation for your platform.
    let mut watcher = RecommendedWatcher::new(tx, Config::default())?;

    // Add a path to be watched. All files and directories at that path and
    // below will be monitored for changes.
    watcher.watch(Path::new(path), RecursiveMode::Recursive)?;

    // Processing Loop
    // For a real CLI, we might want to use a debounce logic (notify-debouncer-mini)
    // For MVP, raw events are okay, but we should handle "Notice: Write"
    
    // Note: notify's receiver is blocking. We can spawn a thread or use specific async bridges.
    // Here we just block the main thread as "watching" is the only task.
    
    // To mix async upload with blocking watch, we can use a runtime handle.
    let rt = tokio::runtime::Handle::current();
    let scan_client = ApiClient::new(&config.backend_url);

    for res in rx {
        match res {
            Ok(event) => {
                match event.kind {
                    notify::EventKind::Create(_) | notify::EventKind::Modify(_) => {
                        for path in event.paths {
                            // Simple ignore check for MVP: Don't touch .git
                            if path.is_file() && !path.components().any(|c| c.as_os_str() == ".git") {
                                // Offload to async
                                let c = ApiClient::new(&config.backend_url); // Cheap clone if refactored, for now new
                                let o = oid.clone();
                                rt.spawn(async move {
                                     // TODO: Ignore check logic needs to be exposed from Scanner 
                                     // For now, simpler check or re-check
                                     println!("Change detected: {:?}", path);
                                     let _ = c.index_file(&path, &o).await;
                                });
                            }
                        }
                    },
                    _ => (),
                }
            },
            Err(e) => println!("Watch error: {:?}", e),
        }
    }

    Ok(())
}
