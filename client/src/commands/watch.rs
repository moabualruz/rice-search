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
    let root = std::fs::canonicalize(path).unwrap_or_else(|_| Path::new(path).to_path_buf());
    println!("Starting watcher on: {:?}", root);

    // Build Ignore Matcher
    // We want to respect .gitignore and .riceignore (if any)
    let mut builder = ignore::gitignore::GitignoreBuilder::new(&root);
    builder.add(root.join(".gitignore"));
    builder.add(root.join(".riceignore"));
    let ignore_matcher = builder.build().unwrap();

    let (tx, rx) = channel();

    // Automatically select the best implementation for your platform.
    let mut watcher = RecommendedWatcher::new(tx, Config::default())?;

    // Add a path to be watched. All files and directories at that path and
    // below will be monitored for changes.
    watcher.watch(&root, RecursiveMode::Recursive)?;

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
                        for raw_path in event.paths {
                            if !raw_path.is_file() { continue; }

                            // Canonicalize to match builder expectation if needed, 
                            // or just ensure we treat them consistently.
                            // Notify usually returns absolute paths.
                            
                            // 1. Ignore .git explicitly (always)
                            if raw_path.components().any(|c| c.as_os_str() == ".git") { continue; }

                            // 2. Check ignores
                            // ignore crate expects path to be relative to root OR absolute if root was absolute
                            match ignore_matcher.matched(&raw_path, false) {
                                ignore::Match::Ignore(_) => {
                                    // println!("Ignored: {:?}", raw_path);
                                    continue; 
                                },
                                _ => {}
                            }

                            // Offload to async
                            let c = ApiClient::new(&config.backend_url); // Cheap clone if refactored, for now new
                            let o = oid.clone();
                            let path_to_send = raw_path.clone(); // Clone for async block
                            let root_for_async = root.clone();

                            rt.spawn(async move {
                                // Calculate hash for audit/log
                                let hash = crate::core::hashing::compute_file_hash(&path_to_send)
                                    .unwrap_or_else(|_| "unknown".to_string());
                                    
                                // Calculate relative path + normalize separators for backend
                                let relative = path_to_send.strip_prefix(&root_for_async).unwrap_or(&path_to_send);
                                let upload_name = relative.to_string_lossy().replace("\\", "/");

                                println!("Change detected: {:?} (hash: {}) -> {}", path_to_send, &hash[..8], upload_name);
                                let _ = c.index_file(&path_to_send, &upload_name, &o).await;
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
