use crate::core::api::ApiClient;
use colored::*;
use ignore::WalkBuilder;
use log::{debug, info, warn};
use std::path::Path;

pub struct Scanner {
    client: ApiClient,
    org_id: String,
}

impl Scanner {
    pub fn new(client: ApiClient, org_id: String) -> Self {
        Self { client, org_id }
    }

    pub async fn scan(&self, path: &Path) {
        // Use the path as provided (relative) - WalkBuilder handles gitignore properly
        info!("Starting initial scan of: {:?}", path);

        let walker = WalkBuilder::new(path)
            .hidden(false) 
            .ignore(true)        // Respect .ignore files
            .git_ignore(true)    // Respect .gitignore
            .add_custom_ignore_filename(".riceignore")
            .filter_entry(|entry| {
                // Only filter .git explicitly, let gitignore handle the rest
                entry.file_name() != ".git"
            })
            .build();

        for result in walker {
            match result {
                Ok(entry) => {
                    let entry_path = entry.path();
                    if entry_path.is_file() {
                        self.process_file(entry_path).await;
                    }
                }
                Err(err) => warn!("Error walking path: {}", err),
            }
        }
        info!("Scan complete.");
    }

    async fn process_file(&self, path: &Path) {
        // Get relative path for display
        let rel_display = path.to_string_lossy().replace("\\", "/");
        debug!("Processing: {}", rel_display);

        // Only resolve to absolute when sending to server
        let abs_path = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
        
        // Clean UNC prefix for server
        let abs_str = abs_path.to_string_lossy();
        let clean_path = if abs_str.starts_with("\\\\?\\") {
            &abs_str[4..]
        } else {
            &abs_str
        };
        let upload_name = clean_path.replace("\\", "/");

        println!("{} {}", "[INDEXING]".blue(), rel_display);

        match self.client.index_file(&abs_path, &upload_name, &self.org_id).await {
            Ok(_) => println!("{} {}", "[OK]".green(), rel_display),
            Err(e) => println!("{} {} ({})", "[ERROR]".red(), rel_display, e),
        }
    }
}
