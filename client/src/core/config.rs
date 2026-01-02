use anyhow::Result;
use config::{Config, File};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct AppConfig {
    pub backend_url: String,
    pub user_id: String,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            backend_url: "http://localhost:8000".to_string(),
            user_id: "default-user".to_string(), // TODO: Generate UUID
        }
    }
}

pub fn load_config() -> Result<AppConfig> {
    let config_dir = dirs::config_dir().unwrap_or_else(|| PathBuf::from("."));
    let config_path = config_dir.join("ricesearch").join("config.toml");

    let s = Config::builder()
        .add_source(File::from(config_path).required(false))
        .add_source(config::Environment::with_prefix("RICE")) // e.g. RICE_BACKEND_URL
        .build()?;

    // If empty/missing, fallback to default manually or let serde handle defaults if wrapped.
    // For now simple deserialization:
    match s.try_deserialize() {
        Ok(c) => Ok(c),
        Err(_) => Ok(AppConfig::default()),
    }
}
