//! Search utilities and helpers
//! 
//! Additional search functionality beyond basic BM25.

use serde::{Deserialize, Serialize};

/// Search configuration options
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SearchConfig {
    /// Maximum number of results to return
    pub limit: usize,
    
    /// Minimum score threshold (0.0 - 1.0)
    pub min_score: Option<f32>,
    
    /// Whether to highlight matches
    pub highlight: bool,
}

impl Default for SearchConfig {
    fn default() -> Self {
        Self {
            limit: 10,
            min_score: None,
            highlight: false,
        }
    }
}

/// Filter results by minimum score
pub fn filter_by_score(results: Vec<(String, f32)>, min_score: f32) -> Vec<(String, f32)> {
    results
        .into_iter()
        .filter(|(_, score)| *score >= min_score)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_filter_by_score() {
        let results = vec![
            ("a".to_string(), 0.9),
            ("b".to_string(), 0.5),
            ("c".to_string(), 0.3),
        ];
        
        let filtered = filter_by_score(results, 0.4);
        assert_eq!(filtered.len(), 2);
    }
}
