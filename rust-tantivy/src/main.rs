//! Rice Search - BM25 Search Service
//! 
//! Standalone Rust service for lexical BM25 search using Tantivy.
//! Provides HTTP API for indexing and searching text chunks.

mod index;
mod search;

use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::IntoResponse,
    routing::{delete, get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tokio::sync::RwLock;
use tower_http::cors::{Any, CorsLayer};
use tower_http::trace::TraceLayer;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use crate::index::TantivyIndex;
use crate::search::{filter_by_score, SearchConfig};

/// Application state shared across handlers
struct AppState {
    index: RwLock<TantivyIndex>,
}

// ============================================================================
// Request/Response Types
// ============================================================================

#[derive(Debug, Deserialize)]
struct IndexRequest {
    chunk_id: String,
    text: String,
}

#[derive(Debug, Deserialize)]
struct BatchIndexRequest {
    chunks: Vec<IndexRequest>,
}

#[derive(Debug, Deserialize)]
struct SearchRequest {
    query: String,
    #[serde(flatten)]
    config: Option<SearchConfig>,
    // Legacy fields for backward compatibility
    limit: Option<usize>,
    min_score: Option<f32>,
}

#[derive(Debug, Serialize)]
struct SearchResult {
    chunk_id: String,
    score: f32,
}

#[derive(Debug, Serialize)]
struct SearchResponse {
    results: Vec<SearchResult>,
    query: String,
    total_hits: usize,
}

#[derive(Debug, Serialize)]
struct HealthResponse {
    status: String,
    indexed_docs: u64,
}

#[derive(Debug, Serialize)]
struct IndexResponse {
    status: String,
    indexed: usize,
}

// ============================================================================
// Handlers
// ============================================================================

/// Health check endpoint
async fn health(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let index = state.index.read().await;
    let doc_count = index.doc_count();
    
    Json(HealthResponse {
        status: "healthy".to_string(),
        indexed_docs: doc_count,
    })
}

/// Index a single chunk
async fn index_chunk(
    State(state): State<Arc<AppState>>,
    Json(req): Json<IndexRequest>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let mut index = state.index.write().await;
    
    index
        .add_document(&req.chunk_id, &req.text)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    index
        .commit()
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    Ok(Json(IndexResponse {
        status: "success".to_string(),
        indexed: 1,
    }))
}

/// Index multiple chunks in batch
async fn batch_index(
    State(state): State<Arc<AppState>>,
    Json(req): Json<BatchIndexRequest>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let mut index = state.index.write().await;
    let count = req.chunks.len();
    
    for chunk in req.chunks {
        index
            .add_document(&chunk.chunk_id, &chunk.text)
            .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    }
    
    index
        .commit()
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    Ok(Json(IndexResponse {
        status: "success".to_string(),
        indexed: count,
    }))
}

/// Search for chunks using BM25
async fn search_chunks(
    State(state): State<Arc<AppState>>,
    Json(req): Json<SearchRequest>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let index = state.index.read().await;

    // Use config if provided, otherwise use legacy fields
    let config = req.config.unwrap_or_else(|| SearchConfig {
        limit: req.limit.unwrap_or(10),
        min_score: req.min_score,
        highlight: false,
    });

    let mut results = index
        .search(&req.query, config.limit)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    // Apply minimum score filter if specified
    if let Some(min_score) = config.min_score {
        results = filter_by_score(results, min_score);
    }

    let search_results: Vec<SearchResult> = results
        .iter()
        .map(|(chunk_id, score)| SearchResult {
            chunk_id: chunk_id.clone(),
            score: *score,
        })
        .collect();

    let total = search_results.len();

    Ok(Json(SearchResponse {
        results: search_results,
        query: req.query,
        total_hits: total,
    }))
}

/// Delete a chunk from the index
async fn delete_chunk(
    State(state): State<Arc<AppState>>,
    Path(chunk_id): Path<String>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let mut index = state.index.write().await;
    
    index
        .delete_document(&chunk_id)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    index
        .commit()
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    Ok(Json(serde_json::json!({
        "status": "deleted",
        "chunk_id": chunk_id
    })))
}

/// Clear the entire index
async fn clear_index(
    State(state): State<Arc<AppState>>,
) -> Result<impl IntoResponse, (StatusCode, String)> {
    let mut index = state.index.write().await;
    
    index
        .clear()
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    
    Ok(Json(serde_json::json!({
        "status": "cleared"
    })))
}

// ============================================================================
// Main
// ============================================================================

#[tokio::main]
async fn main() {
    // Initialize tracing
    tracing_subscriber::registry()
        .with(tracing_subscriber::EnvFilter::new(
            std::env::var("RUST_LOG").unwrap_or_else(|_| "info".into()),
        ))
        .with(tracing_subscriber::fmt::layer())
        .init();

    // Get configuration from environment
    let data_dir = std::env::var("TANTIVY_DATA_DIR").unwrap_or_else(|_| "./data".to_string());
    let port = std::env::var("PORT").unwrap_or_else(|_| "3002".to_string());
    let host = std::env::var("HOST").unwrap_or_else(|_| "0.0.0.0".to_string());

    tracing::info!("Initializing Tantivy index at {}", data_dir);

    // Create index
    let tantivy_index = TantivyIndex::new(&data_dir).expect("Failed to create Tantivy index");

    let state = Arc::new(AppState {
        index: RwLock::new(tantivy_index),
    });

    // Build router
    let app = Router::new()
        .route("/health", get(health))
        .route("/index", post(index_chunk))
        .route("/index/batch", post(batch_index))
        .route("/index/{chunk_id}", delete(delete_chunk))
        .route("/index/clear", post(clear_index))
        .route("/search", post(search_chunks))
        .layer(CorsLayer::new().allow_origin(Any).allow_methods(Any))
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let addr = format!("{}:{}", host, port);
    tracing::info!("Starting BM25 search service on {}", addr);

    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}
