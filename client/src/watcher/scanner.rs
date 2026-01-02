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
        // Canonicalize root for consistent relative paths
        let root = std::fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
        info!("Starting initial scan of: {:?}", root);

        let walker = WalkBuilder::new(&root)
            .hidden(false) 
            .ignore(true)
            .git_ignore(true)
            .add_custom_ignore_filename(".riceignore")
            .filter_entry(|entry| entry.file_name() != ".git")
            .build();

        for result in walker {
            match result {
                Ok(entry) => {
                    let entry_path = entry.path();
                    if entry_path.is_file() {
                        self.process_file(entry_path, &root).await;
                    }
                }
                Err(err) => warn!("Error walking path: {}", err),
            }
        }
        info!("Scan complete.");
    }

    async fn process_file(&self, path: &Path, root: &Path) {
        let path_str = path.display().to_string();
        debug!("Processing: {}", path_str);

        // TODO: Hash check optimization could go here
        
        // Calculate relative path for upload name
        // If path is absolute and root is relative (e.g. "."), we might need canonicalization.
        // Use path diff or just assume they are compatible if from WalkBuilder.
        let relative = path.strip_prefix(root).unwrap_or(path);
        let upload_name = relative.to_string_lossy().replace("\\", "/");

        println!("{} {}", "[INDEXING]".blue(), upload_name);

        match self.client.index_file(path, &upload_name, &self.org_id).await {
            Ok(_) => println!("{} {}", "[OK]".green(), upload_name),
            Err(e) => println!("{} {} ({})", "[ERROR]".red(), upload_name, e),
        }
    }
}
