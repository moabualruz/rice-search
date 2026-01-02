use crate::core::api::ApiClient;
use crate::core::config::load_config;
use anyhow::Result;
use colored::*;

pub async fn run(query: &str, limit: usize, json: bool) -> Result<()> {
    let config = load_config()?;
    let client = ApiClient::new(&config.backend_url);

    let result = client.search(query, limit, true).await?;

    if json {
        println!("{}", serde_json::to_string_pretty(&result)?);
        return Ok(());
    }

    // Pretty Print
    if let Some(results) = result.get("results").and_then(|v| v.as_array()) {
        if results.is_empty() {
            println!("No results found.");
            return Ok(());
        }

        for item in results {
            let path = item
                .get("path")
                .and_then(|s| s.as_str())
                .unwrap_or("unknown");
            let line = item.get("start_line").and_then(|n| n.as_u64()).unwrap_or(0);
            let snippet = item.get("content").and_then(|s| s.as_str()).unwrap_or("");
            let score = item.get("score").and_then(|f| f.as_f64()).unwrap_or(0.0);

            println!(
                "{}:{}:{:.4}",
                path.magenta(),
                line.to_string().green(),
                score
            );
            for l in snippet.lines().take(3) {
                // Limit snippet lines
                println!("  {}", l.trim().dimmed());
            }
            println!();
        }
    } else {
        println!("Invalid response format.");
    }

    Ok(())
}
