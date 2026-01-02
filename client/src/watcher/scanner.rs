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
        info!("Starting initial scan of: {:?}", path);

        let walker = WalkBuilder::new(path)
            .hidden(false) // Allow hidden files (like .github)
            .ignore(true)
            .git_ignore(true)
            .add_custom_ignore_filename(".riceignore") // Support .riceignore
            .filter_entry(|entry| entry.file_name() != ".git") // Explicitly exclude .git dir
            .build();

        for result in walker {
            match result {
                Ok(entry) => {
                    let path = entry.path();
                    if path.is_file() {
                        self.process_file(path).await;
                    }
                }
                Err(err) => warn!("Error walking path: {}", err),
            }
        }
        info!("Scan complete.");
    }

    async fn process_file(&self, path: &Path) {
        let path_str = path.display().to_string();
        debug!("Processing: {}", path_str);

        // TODO: Hash check optimization could go here (store local state DB)
        // For now, we trust the backend to dedup or just re-upload (less efficient but simpler MVP)

        println!("{} {}", "[INDEXING]".blue(), path_str);

        match self.client.index_file(path, &self.org_id).await {
            Ok(_) => println!("{} {}", "[OK]".green(), path_str),
            Err(e) => println!("{} {} ({})", "[ERROR]".red(), path_str, e),
        }
    }
}
