use crate::core::api::ApiClient;
use crate::core::config::load_config;
use crate::watcher::scanner::Scanner;
use anyhow::Result;
use colored::*;
use notify::{Config, RecommendedWatcher, RecursiveMode, Watcher};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::mpsc::channel;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

const DEBOUNCE_DELAY: Duration = Duration::from_secs(3);

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

    println!("Starting watcher on: {} (debounce: {}s)", path, DEBOUNCE_DELAY.as_secs());

    let (tx, rx) = channel();

    let mut watcher = RecommendedWatcher::new(tx, Config::default())?;

    // Watch the path as provided
    watcher.watch(root_path, RecursiveMode::Recursive)?;

    let rt = tokio::runtime::Handle::current();
    
    // Per-file debounce tracking: file_path -> (last_change_time, scheduled)
    let pending_files: Arc<Mutex<HashMap<PathBuf, Instant>>> = Arc::new(Mutex::new(HashMap::new()));
    
    // Spawn debounce processor
    let pending_clone = pending_files.clone();
    let config_clone = config.clone();
    let oid_clone = oid.clone();
    
    rt.spawn(async move {
        loop {
            tokio::time::sleep(Duration::from_millis(500)).await;
            
            // Check for files ready to be indexed
            let files_ready: Vec<PathBuf> = {
                let mut pending = pending_clone.lock().unwrap();
                let now = Instant::now();
                let ready: Vec<PathBuf> = pending
                    .iter()
                    .filter(|(_, last_change)| now.duration_since(**last_change) >= DEBOUNCE_DELAY)
                    .map(|(path, _)| path.clone())
                    .collect();
                
                // Remove ready files from pending
                for path in &ready {
                    pending.remove(path);
                }
                ready
            };
            
            // Index ready files
            for file_path in files_ready {
                let c = ApiClient::new(&config_clone.backend_url);
                let o = oid_clone.clone();
                
                let abs_path = std::fs::canonicalize(&file_path)
                    .unwrap_or_else(|_| file_path.clone());
                
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
            }
        }
    });

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
                            let cwd = std::env::current_dir().unwrap_or_default();
                            let relative_path = event_path.strip_prefix(&cwd)
                                .or_else(|_| event_path.strip_prefix(root_path))
                                .unwrap_or(&event_path);
                            
                            // Normalize to forward slashes for gitignore matching
                            let rel_str = relative_path.to_string_lossy().replace("\\", "/");
                            
                            // 3. Check gitignore
                            let matched = ignore_matcher.matched_path_or_any_parents(&rel_str, false);
                            
                            match matched {
                                ignore::Match::Ignore(_) => {
                                    continue; 
                                },
                                _ => {}
                            }

                            // 4. Add/update to pending (debounce)
                            {
                                let mut pending = pending_files.lock().unwrap();
                                pending.insert(event_path.clone(), Instant::now());
                            }
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
