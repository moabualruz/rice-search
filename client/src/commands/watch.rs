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

    // Use the path as provided (relative like ".")
    let root_path = Path::new(path);
    
    let scanner = Scanner::new(ApiClient::new(&config.backend_url), oid.clone());

    // Initial Scan
    if full_index {
        scanner.scan(root_path).await;
    }

    // Build Ignore Matcher from the root path (works with relative paths)
    let mut builder = ignore::gitignore::GitignoreBuilder::new(root_path);
    builder.add(root_path.join(".gitignore"));
    builder.add(root_path.join(".riceignore"));
    let ignore_matcher = builder.build().unwrap();

    println!("Starting watcher on: {}", path);

    let (tx, rx) = channel();

    let mut watcher = RecommendedWatcher::new(tx, Config::default())?;

    // Watch the path as provided
    watcher.watch(root_path, RecursiveMode::Recursive)?;

    let rt = tokio::runtime::Handle::current();

    for res in rx {
        match res {
            Ok(event) => {
                match event.kind {
                    notify::EventKind::Create(_) | notify::EventKind::Modify(_) => {
                        for event_path in event.paths {
                            if !event_path.is_file() { continue; }

                            // 1. Ignore .git explicitly
                            if event_path.components().any(|c| c.as_os_str() == ".git") { continue; }

                            // 2. Get relative path from the event path
                            // Notify returns absolute paths, so we need to strip the current dir
                            let cwd = std::env::current_dir().unwrap_or_default();
                            let relative_path = event_path.strip_prefix(&cwd)
                                .or_else(|_| event_path.strip_prefix(root_path))
                                .unwrap_or(&event_path);
                            
                            // Normalize to forward slashes for gitignore matching
                            let rel_str = relative_path.to_string_lossy().replace("\\", "/");
                            
                            // 3. Check gitignore - also check parent directories
                            // For path like "backend/tmp/file.txt", check if "backend/tmp" is ignored
                            let matched = ignore_matcher.matched_path_or_any_parents(&rel_str, false);
                            
                            match matched {
                                ignore::Match::Ignore(_) => {
                                    continue; 
                                },
                                _ => {}
                            }

                            // 4. Passed ignore check - send to server with ABSOLUTE path
                            let c = ApiClient::new(&config.backend_url);
                            let o = oid.clone();
                            let abs_path = std::fs::canonicalize(&event_path)
                                .unwrap_or_else(|_| event_path.clone());

                            rt.spawn(async move {
                                let hash = crate::core::hashing::compute_file_hash(&abs_path)
                                    .unwrap_or_else(|_| "unknown".to_string());
                                
                                // Clean UNC prefix for server
                                let path_str = abs_path.to_string_lossy();
                                let clean_path = if path_str.starts_with("\\\\?\\") {
                                    &path_str[4..]
                                } else {
                                    &path_str
                                };
                                let upload_name = clean_path.replace("\\", "/");

                                println!("Indexing: {} (hash: {})", upload_name, &hash[..8]);
                                let _ = c.index_file(&abs_path, &upload_name, &o).await;
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
