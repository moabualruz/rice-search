//! Tantivy Index Management
//! 
//! Handles creation, modification, and persistence of the BM25 index.

use std::path::Path;
use tantivy::{
    directory::MmapDirectory,
    schema::{Schema, Value, STORED, STRING, TEXT},
    Index, IndexWriter, TantivyDocument,
};
use thiserror::Error;

/// Errors that can occur during index operations
#[derive(Error, Debug)]
pub enum IndexError {
    #[error("Tantivy error: {0}")]
    Tantivy(#[from] tantivy::TantivyError),
    
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    
    #[error("Query parse error: {0}")]
    QueryParse(#[from] tantivy::query::QueryParserError),
    
    #[error("Directory error: {0}")]
    Directory(#[from] tantivy::directory::error::OpenDirectoryError),
}

/// Wrapper around Tantivy index for BM25 search
pub struct TantivyIndex {
    index: Index,
    writer: IndexWriter,
    chunk_id_field: tantivy::schema::Field,
    text_field: tantivy::schema::Field,
}

impl TantivyIndex {
    /// Create or open a Tantivy index at the specified path
    pub fn new(data_dir: &str) -> Result<Self, IndexError> {
        let path = Path::new(data_dir);
        
        // Create directory if it doesn't exist
        std::fs::create_dir_all(path)?;
        
        // Build schema
        let mut schema_builder = Schema::builder();
        let chunk_id_field = schema_builder.add_text_field("chunk_id", STRING | STORED);
        let text_field = schema_builder.add_text_field("text", TEXT);
        let schema = schema_builder.build();
        
        // Open or create index
        let index = if path.join("meta.json").exists() {
            // Open existing index
            let dir = MmapDirectory::open(path)?;
            Index::open(dir)?
        } else {
            // Create new index
            let dir = MmapDirectory::open(path)?;
            Index::create(dir, schema.clone(), tantivy::IndexSettings::default())?
        };
        
        // Create writer with 50MB buffer
        let writer = index.writer(50_000_000)?;
        
        Ok(Self {
            index,
            writer,
            chunk_id_field,
            text_field,
        })
    }
    
    /// Add a document to the index (not committed until commit() is called)
    pub fn add_document(&mut self, chunk_id: &str, text: &str) -> Result<(), IndexError> {
        // Delete existing document with same chunk_id first
        self.delete_document(chunk_id)?;
        
        let mut doc = TantivyDocument::default();
        doc.add_text(self.chunk_id_field, chunk_id);
        doc.add_text(self.text_field, text);
        
        self.writer.add_document(doc)?;
        Ok(())
    }
    
    /// Delete a document by chunk_id
    pub fn delete_document(&mut self, chunk_id: &str) -> Result<(), IndexError> {
        let term = tantivy::Term::from_field_text(self.chunk_id_field, chunk_id);
        self.writer.delete_term(term);
        Ok(())
    }
    
    /// Commit pending changes to disk
    pub fn commit(&mut self) -> Result<(), IndexError> {
        self.writer.commit()?;
        Ok(())
    }
    
    /// Clear the entire index
    pub fn clear(&mut self) -> Result<(), IndexError> {
        self.writer.delete_all_documents()?;
        self.writer.commit()?;
        Ok(())
    }
    
    /// Get the number of documents in the index
    pub fn doc_count(&self) -> u64 {
        let reader = self.index.reader().ok();
        reader.map(|r| r.searcher().num_docs()).unwrap_or(0)
    }
    
    /// Search for documents using BM25
    pub fn search(&self, query_str: &str, limit: usize) -> Result<Vec<(String, f32)>, IndexError> {
        use tantivy::collector::TopDocs;
        use tantivy::query::QueryParser;
        
        let reader = self.index.reader()?;
        let searcher = reader.searcher();
        
        // Build query parser for text field
        let query_parser = QueryParser::for_index(&self.index, vec![self.text_field]);
        let query = query_parser.parse_query(query_str)?;
        
        // Execute search
        let top_docs = searcher.search(&query, &TopDocs::with_limit(limit))?;
        
        // Extract results
        let mut results = Vec::with_capacity(top_docs.len());
        for (score, doc_address) in top_docs {
            let doc: TantivyDocument = searcher.doc(doc_address)?;
            if let Some(chunk_id_value) = doc.get_first(self.chunk_id_field) {
                // Extract string from CompactDocValue (Tantivy 0.25+)
                if let Some(text) = chunk_id_value.as_str() {
                    results.push((text.to_string(), score));
                }
            }
        }
        
        Ok(results)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    
    #[test]
    fn test_index_and_search() {
        let temp_dir = TempDir::new().unwrap();
        let mut index = TantivyIndex::new(temp_dir.path().to_str().unwrap()).unwrap();
        
        // Index some documents
        index.add_document("chunk1", "hello world rust programming").unwrap();
        index.add_document("chunk2", "python machine learning").unwrap();
        index.add_document("chunk3", "rust systems programming").unwrap();
        index.commit().unwrap();
        
        // Search
        let results = index.search("rust", 10).unwrap();
        assert_eq!(results.len(), 2);
        
        // First result should be about rust
        assert!(results[0].0 == "chunk1" || results[0].0 == "chunk3");
    }
    
    #[test]
    fn test_delete_document() {
        let temp_dir = TempDir::new().unwrap();
        let mut index = TantivyIndex::new(temp_dir.path().to_str().unwrap()).unwrap();
        
        index.add_document("chunk1", "hello world").unwrap();
        index.commit().unwrap();
        
        assert_eq!(index.doc_count(), 1);
        
        index.delete_document("chunk1").unwrap();
        index.commit().unwrap();
        
        assert_eq!(index.doc_count(), 0);
    }
}
