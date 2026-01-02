use crate::core::api::ApiClient;
use crate::core::config::load_config;
use crate::watcher::scanner::Scanner;
use anyhow::Result;
use colored::*;
use notify::{Config, RecommendedWatcher, RecursiveMode, Watcher};
use std::path::Path;
use std::sync::mpsc::channel;

pub async fn run(path: &str, org_id: Option<String>, full_index: bool) -> Result<()> {
    let config = load_config()?;
    let client = ApiClient::new(&config.backend_url);
    
    // Check health before starting
    if !client.health_check().await {
        eprintln!("{} Backend at {} seems down or unhealthy.", "Warning:".yellow(), config.backend_url);
    } else {
        println!("{} Backend connected successfully.", "✓".green());
    }

    let oid = org_id.unwrap_or("public".to_string());

    let scanner = Scanner::new(ApiClient::new(&config.backend_url), oid.clone()); // Clone for scanner

    // Initial Scan
    if full_index {
        scanner.scan(Path::new(path)).await;
    }

    // Watcher Setup
    println!("Starting watcher on: {}", path);

    // Build Ignore Matcher
    // We want to respect .gitignore and .riceignore (if any)
    let mut builder = ignore::gitignore::GitignoreBuilder::new(path);
    builder.add(Path::new(path).join(".gitignore"));
    builder.add(Path::new(path).join(".riceignore"));
    let ignore_matcher = builder.build().unwrap();

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

    for res in rx {
        match res {
            Ok(event) => {
                match event.kind {
                    notify::EventKind::Create(_) | notify::EventKind::Modify(_) => {
                        for path in event.paths {
                            if !path.is_file() { continue; }

                            // 1. Ignore .git explicitly (always)
                            if path.components().any(|c| c.as_os_str() == ".git") { continue; }

                            // 2. Check ignores
                            // ignore crate expects relative path mostly, or full path if built with root
                            // match returns Ignore, Allow, or None
                            match ignore_matcher.matched(&path, false) {
                                ignore::Match::Ignore(_) => {
                                    // println!("Ignored: {:?}", path);
                                    continue; 
                                },
                                _ => {}
                            }

                            // Offload to async
                            let c = ApiClient::new(&config.backend_url); // Cheap clone if refactored, for now new
                            let o = oid.clone();
                            rt.spawn(async move {
                                // Calculate hash for audit/log
                                let hash = crate::core::hashing::compute_file_hash(&path)
                                    .unwrap_or_else(|_| "unknown".to_string());
                                    
                                println!("Change detected: {:?} (hash: {})", path, &hash[..8]);
                                let _ = c.index_file(&path, &o).await;
                            });
                        }
                    }
                    _ => (),
                }
            }
            Err(e) => println!("Watch error: {:?}", e),
        }
    }

    Ok(())
}
