use anyhow::{Context, Result};
use clap::{Parser, Subcommand};
use serde::{Deserialize, Serialize};
use std::fs;
use std::io::{self, BufRead};
use std::path::PathBuf;
use tantivy::collector::TopDocs;
use tantivy::query::QueryParser;
use tantivy::schema::*;
use tantivy::{doc, Index, IndexWriter, ReloadPolicy, Searcher, Term, TantivyDocument};

/// Tantivy CLI for code search indexing and querying
#[derive(Parser)]
#[command(name = "tantivy-cli")]
#[command(about = "Sparse search service using Tantivy")]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Index documents from stdin (JSON lines)
    Index {
        /// Path to the index directory
        #[arg(short, long)]
        index_path: PathBuf,
        /// Store name
        #[arg(short, long)]
        store: String,
    },
    /// Search the index
    Search {
        /// Path to the index directory
        #[arg(short, long)]
        index_path: PathBuf,
        /// Store name
        #[arg(short, long)]
        store: String,
        /// Search query
        #[arg(short, long)]
        query: String,
        /// Maximum results
        #[arg(short = 'k', long, default_value = "200")]
        top_k: usize,
        /// Path prefix filter
        #[arg(long)]
        path_prefix: Option<String>,
        /// Language filter
        #[arg(long)]
        language: Option<String>,
    },
    /// Delete documents by path or doc_id
    Delete {
        /// Path to the index directory
        #[arg(short, long)]
        index_path: PathBuf,
        /// Store name
        #[arg(short, long)]
        store: String,
        /// Delete by path prefix
        #[arg(long)]
        path: Option<String>,
        /// Delete by doc_id
        #[arg(long)]
        doc_id: Option<String>,
    },
    /// Get index statistics
    Stats {
        /// Path to the index directory
        #[arg(short, long)]
        index_path: PathBuf,
        /// Store name
        #[arg(short, long)]
        store: String,
    },
}

/// Document to be indexed
#[derive(Debug, Deserialize)]
struct InputDocument {
    doc_id: String,
    path: String,
    language: String,
    symbols: Vec<String>,
    content: String,
    start_line: u64,
    end_line: u64,
}

/// Search result
#[derive(Debug, Serialize)]
struct SearchResult {
    doc_id: String,
    path: String,
    language: String,
    symbols: Vec<String>,
    content: String,
    start_line: u64,
    end_line: u64,
    bm25_score: f32,
    rank: usize,
}

/// Index statistics
#[derive(Debug, Serialize)]
struct IndexStats {
    store: String,
    num_docs: u64,
    num_segments: usize,
}

/// Schema fields for code search
struct CodeSchema {
    schema: Schema,
    doc_id: Field,
    store_field: Field,
    path: Field,
    language: Field,
    symbols: Field,
    content: Field,
    start_line: Field,
    end_line: Field,
}

impl CodeSchema {
    fn new() -> Self {
        let mut schema_builder = Schema::builder();

        // doc_id: unique identifier, stored and indexed as STRING (exact match)
        let doc_id = schema_builder.add_text_field("doc_id", STRING | STORED);

        // store: store name for multi-store support
        let store_field = schema_builder.add_text_field("store", STRING | STORED);

        // path: file path, indexed for prefix queries and full-text
        let path_options = TextOptions::default()
            .set_stored()
            .set_indexing_options(
                TextFieldIndexing::default()
                    .set_tokenizer("default")
                    .set_index_option(IndexRecordOption::WithFreqsAndPositions),
            );
        let path = schema_builder.add_text_field("path", path_options);

        // language: programming language, stored and indexed as STRING
        let language = schema_builder.add_text_field("language", STRING | STORED);

        // symbols: function/class names, indexed for boosted matching
        let symbols_options = TextOptions::default()
            .set_stored()
            .set_indexing_options(
                TextFieldIndexing::default()
                    .set_tokenizer("default")
                    .set_index_option(IndexRecordOption::WithFreqsAndPositions),
            );
        let symbols = schema_builder.add_text_field("symbols", symbols_options);

        // content: code content, full-text indexed
        let content_options = TextOptions::default()
            .set_stored()
            .set_indexing_options(
                TextFieldIndexing::default()
                    .set_tokenizer("default")
                    .set_index_option(IndexRecordOption::WithFreqsAndPositions),
            );
        let content = schema_builder.add_text_field("content", content_options);

        // Line numbers
        let start_line = schema_builder.add_u64_field("start_line", STORED | FAST);
        let end_line = schema_builder.add_u64_field("end_line", STORED | FAST);

        CodeSchema {
            schema: schema_builder.build(),
            doc_id,
            store_field,
            path,
            language,
            symbols,
            content,
            start_line,
            end_line,
        }
    }
}

fn get_or_create_index(index_path: &PathBuf, schema: &Schema) -> Result<Index> {
    if index_path.exists() {
        Index::open_in_dir(index_path).context("Failed to open existing index")
    } else {
        fs::create_dir_all(index_path).context("Failed to create index directory")?;
        Index::create_in_dir(index_path, schema.clone()).context("Failed to create index")
    }
}

fn index_documents(index_path: PathBuf, store: String) -> Result<()> {
    let code_schema = CodeSchema::new();
    let index = get_or_create_index(&index_path, &code_schema.schema)?;

    // 50MB heap for writer
    let mut index_writer: IndexWriter = index.writer(50_000_000)?;

    let stdin = io::stdin();
    let mut count = 0;
    let mut errors = 0;

    for line in stdin.lock().lines() {
        let line = line.context("Failed to read line from stdin")?;
        if line.trim().is_empty() {
            continue;
        }

        match serde_json::from_str::<InputDocument>(&line) {
            Ok(doc) => {
                // Delete existing document with same doc_id (upsert)
                let term = Term::from_field_text(code_schema.doc_id, &doc.doc_id);
                index_writer.delete_term(term);

                // Add new document
                let symbols_text = doc.symbols.join(" ");
                index_writer.add_document(doc!(
                    code_schema.doc_id => doc.doc_id,
                    code_schema.store_field => store.clone(),
                    code_schema.path => doc.path,
                    code_schema.language => doc.language,
                    code_schema.symbols => symbols_text,
                    code_schema.content => doc.content,
                    code_schema.start_line => doc.start_line,
                    code_schema.end_line => doc.end_line,
                ))?;
                count += 1;
            }
            Err(e) => {
                eprintln!("Error parsing document: {}", e);
                errors += 1;
            }
        }
    }

    index_writer.commit()?;

    let result = serde_json::json!({
        "indexed": count,
        "errors": errors,
        "store": store
    });
    println!("{}", serde_json::to_string(&result)?);

    Ok(())
}

fn search_index(
    index_path: PathBuf,
    store: String,
    query_str: String,
    top_k: usize,
    path_prefix: Option<String>,
    language: Option<String>,
) -> Result<()> {
    let code_schema = CodeSchema::new();
    let index = Index::open_in_dir(&index_path).context("Failed to open index")?;

    let reader = index
        .reader_builder()
        .reload_policy(ReloadPolicy::OnCommitWithDelay)
        .try_into()?;

    let searcher: Searcher = reader.searcher();

    // Build query with boosted fields
    // Symbols get 3x boost, path gets 2x boost, content gets 1x
    let mut query_parser = QueryParser::for_index(
        &index,
        vec![code_schema.symbols, code_schema.path, code_schema.content],
    );

    // Set field boosts
    query_parser.set_field_boost(code_schema.symbols, 3.0);
    query_parser.set_field_boost(code_schema.path, 2.0);
    query_parser.set_field_boost(code_schema.content, 1.0);

    let query = query_parser
        .parse_query(&query_str)
        .context("Failed to parse query")?;

    let top_docs = searcher
        .search(&query, &TopDocs::with_limit(top_k * 2)) // Get extra for filtering
        .context("Search failed")?;

    let mut results: Vec<SearchResult> = Vec::new();
    let mut rank = 0;

    for (score, doc_address) in top_docs {
        let retrieved_doc: TantivyDocument = searcher.doc(doc_address)?;

        // Extract fields
        let doc_id = retrieved_doc
            .get_first(code_schema.doc_id)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();

        let doc_store = retrieved_doc
            .get_first(code_schema.store_field)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();

        // Filter by store
        if doc_store != store {
            continue;
        }

        let path = retrieved_doc
            .get_first(code_schema.path)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();

        let lang = retrieved_doc
            .get_first(code_schema.language)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();

        // Filter by path prefix
        if let Some(ref prefix) = path_prefix {
            if !path.starts_with(prefix) {
                continue;
            }
        }

        // Filter by language
        if let Some(ref lang_filter) = language {
            if &lang != lang_filter {
                continue;
            }
        }

        let symbols_text = retrieved_doc
            .get_first(code_schema.symbols)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();
        let symbols: Vec<String> = symbols_text
            .split_whitespace()
            .map(|s| s.to_string())
            .collect();

        let content = retrieved_doc
            .get_first(code_schema.content)
            .and_then(|v| v.as_str())
            .unwrap_or("")
            .to_string();

        let start_line = retrieved_doc
            .get_first(code_schema.start_line)
            .and_then(|v| v.as_u64())
            .unwrap_or(0);

        let end_line = retrieved_doc
            .get_first(code_schema.end_line)
            .and_then(|v| v.as_u64())
            .unwrap_or(0);

        rank += 1;
        results.push(SearchResult {
            doc_id,
            path,
            language: lang,
            symbols,
            content,
            start_line,
            end_line,
            bm25_score: score,
            rank,
        });

        if results.len() >= top_k {
            break;
        }
    }

    let output = serde_json::json!({
        "results": results,
        "total": results.len(),
        "query": query_str,
        "store": store
    });
    println!("{}", serde_json::to_string(&output)?);

    Ok(())
}

fn delete_documents(
    index_path: PathBuf,
    store: String,
    path: Option<String>,
    doc_id: Option<String>,
) -> Result<()> {
    let code_schema = CodeSchema::new();
    let index = Index::open_in_dir(&index_path).context("Failed to open index")?;

    let mut index_writer: IndexWriter = index.writer(50_000_000)?;
    let mut deleted = 0;

    if let Some(doc_id_val) = doc_id {
        let term = Term::from_field_text(code_schema.doc_id, &doc_id_val);
        index_writer.delete_term(term);
        deleted += 1;
    }

    if let Some(path_prefix) = path {
        // For path prefix deletion, we need to search and delete
        let reader = index
            .reader_builder()
            .reload_policy(ReloadPolicy::OnCommitWithDelay)
            .try_into()?;

        let searcher = reader.searcher();

        // Get all docs matching store
        let query_parser = QueryParser::for_index(&index, vec![code_schema.store_field]);
        let query = query_parser.parse_query(&format!("\"{}\"", store))?;

        let all_docs = searcher.search(&query, &TopDocs::with_limit(100_000))?;

        for (_score, doc_address) in all_docs {
            let doc: TantivyDocument = searcher.doc(doc_address)?;
            let doc_path = doc
                .get_first(code_schema.path)
                .and_then(|v| v.as_str())
                .unwrap_or("");

            if doc_path.starts_with(&path_prefix) {
                if let Some(id) = doc.get_first(code_schema.doc_id).and_then(|v| v.as_str()) {
                    let term = Term::from_field_text(code_schema.doc_id, id);
                    index_writer.delete_term(term);
                    deleted += 1;
                }
            }
        }
    }

    index_writer.commit()?;

    let result = serde_json::json!({
        "deleted": deleted,
        "store": store
    });
    println!("{}", serde_json::to_string(&result)?);

    Ok(())
}

fn get_stats(index_path: PathBuf, store: String) -> Result<()> {
    let code_schema = CodeSchema::new();
    let index = Index::open_in_dir(&index_path).context("Failed to open index")?;

    let reader = index
        .reader_builder()
        .reload_policy(ReloadPolicy::OnCommitWithDelay)
        .try_into()?;

    let searcher = reader.searcher();

    // Count documents in this store
    let query_parser = QueryParser::for_index(&index, vec![code_schema.store_field]);
    let query = query_parser.parse_query(&format!("\"{}\"", store))?;
    let count = searcher.search(&query, &TopDocs::with_limit(1_000_000))?.len();

    let stats = IndexStats {
        store,
        num_docs: count as u64,
        num_segments: searcher.segment_readers().len(),
    };

    println!("{}", serde_json::to_string(&stats)?);

    Ok(())
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Index { index_path, store } => {
            index_documents(index_path, store)?;
        }
        Commands::Search {
            index_path,
            store,
            query,
            top_k,
            path_prefix,
            language,
        } => {
            search_index(index_path, store, query, top_k, path_prefix, language)?;
        }
        Commands::Delete {
            index_path,
            store,
            path,
            doc_id,
        } => {
            delete_documents(index_path, store, path, doc_id)?;
        }
        Commands::Stats { index_path, store } => {
            get_stats(index_path, store)?;
        }
    }

    Ok(())
}
