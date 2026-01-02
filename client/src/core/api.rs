use anyhow::{Context, Result};
use reqwest::{multipart, Client};
use serde_json::Value;
use std::path::Path;

pub struct ApiClient {
    client: Client,
    base_url: String,
}

impl ApiClient {
    pub fn new(base_url: &str) -> Self {
        Self {
            client: Client::new(),
            base_url: base_url.trim_end_matches('/').to_string(),
        }
    }

    pub async fn health_check(&self) -> bool {
        match self
            .client
            .get(format!("{}/healthz", self.base_url))
            .send()
            .await
        {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }

    pub async fn index_file(&self, path: &Path, org_id: &str) -> Result<Value> {
        // Read file content eagerly for simplicity/robustness (<10MB files usually)
        let content = tokio::fs::read(path).await.context("Failed to read file")?;
        let filename = path
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("unknown")
            .to_string();

        let part = multipart::Part::bytes(content).file_name(filename);
        let form = multipart::Form::new()
            .part("file", part)
            .text("org_id", org_id.to_string());

        let resp = self
            .client
            .post(format!("{}/api/v1/ingest/file", self.base_url))
            .multipart(form)
            .send()
            .await?;

        if !resp.status().is_success() {
            anyhow::bail!("Server returned error: {}", resp.status());
        }

        let json: Value = resp.json().await?;
        Ok(json)
    }

    pub async fn search(&self, query: &str, limit: usize, hybrid: bool) -> Result<Value> {
        let body = serde_json::json!({
            "query": query,
            "mode": "search",
            "hybrid": hybrid,
            "limit": limit
        });

        let resp = self
            .client
            .post(format!("{}/api/v1/search/query", self.base_url))
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            anyhow::bail!("Search failed: {}", resp.status());
        }

        let json: Value = resp.json().await?;
        Ok(json)
    }
}
