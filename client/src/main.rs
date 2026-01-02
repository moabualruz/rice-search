mod core;
mod watcher;
mod commands;

use clap::{Parser, Subcommand};
use anyhow::Result;
use commands::{watch, search};

#[derive(Parser)]
#[command(name = "ricesearch")]
#[command(about = "Rice Search Client - High performance local code search", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Watch a directory and index changes
    Watch {
        /// Directory to watch
        #[arg(default_value = ".")]
        path: String,
        
        /// Organization ID (optional scope)
        #[arg(short, long)]
        org_id: Option<String>,

        /// Perform full initial index
        #[arg(long, short = 'f', default_value_t = false)]
        full_index: bool,
    },
    
    /// Search indexed code
    Search {
        /// Search query
        query: String,

        /// Limit results
        #[arg(short, long, default_value_t = 10)]
        limit: usize,
        
        /// Output as JSON
        #[arg(long, default_value_t = false)]
        json: bool,
    },

    /// Index a directory once (no watch)
    Index {
        /// Directory to index
        #[arg(default_value = ".")]
        path: String,
    },
    
    /// Manage configuration
    Config {
        #[command(subcommand)]
        action: ConfigAction,
    },
}

#[derive(Subcommand)]
enum ConfigAction {
    /// Show current configuration
    Show,
    /// Set a configuration value
    Set { key: String, value: String },
}

#[tokio::main]
async fn main() -> Result<()> {
    // env_logger::init(); // Ensure not initialized twice if we move it or use another logger config
    if std::env::var("RUST_LOG").is_err() {
        std::env::set_var("RUST_LOG", "info");
    }
    env_logger::init();
    
    let cli = Cli::parse();

    match &cli.command {
        Commands::Watch { path, org_id, full_index } => {
            watch::run(path, org_id.clone(), *full_index).await?;
        }
        Commands::Search { query, limit, json } => {
            search::run(query, *limit, *json).await?;
        }
        Commands::Index { path } => {
            // Re-use watch logic but exit after initial scan? 
            // Or explicit scan function.
            // For MVP re-use logic part or just scan:
            // Let's call the scanner directly for Index
            let config = core::config::load_config()?;
            let client = core::api::ApiClient::new(&config.backend_url);
            let scanner = watcher::scanner::Scanner::new(client, "public".to_string());
            scanner.scan(std::path::Path::new(path)).await;
        }
        Commands::Config { action } => {
            match action {
                ConfigAction::Show => {
                     let c = core::config::load_config()?;
                     println!("{:#?}", c);
                },
                ConfigAction::Set { key, value } => println!("Set {} = {} (Not implemented persistence yet)", key, value),
            }
        }
    }

    Ok(())
}
