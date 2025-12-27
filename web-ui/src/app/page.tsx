'use client';

import { useState, FormEvent } from 'react';

interface SearchResult {
  doc_id: string;
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content?: string;
  symbols: string[];
  final_score: number;
}

interface SearchResponse {
  query: string;
  results: SearchResult[];
  total: number;
  store: string;
  search_time_ms: number;
}

export default function Home() {
  const [query, setQuery] = useState('');
  const [pathPrefix, setPathPrefix] = useState('');
  const [store, setStore] = useState('default');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searchTime, setSearchTime] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSearch = async (e: FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setError(null);

    try {
      const response = await fetch('/api/v1/search/' + store, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: query.trim(),
          top_k: 20,
          include_content: true,
          filters: pathPrefix ? { path_prefix: pathPrefix } : undefined,
        }),
      });

      if (!response.ok) {
        throw new Error(`Search failed: ${response.statusText}`);
      }

      const data: SearchResponse = await response.json();
      setResults(data.results);
      setSearchTime(data.search_time_ms);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container">
      <header className="header">
        <h1>Local Code Search</h1>
        <p>Hybrid semantic + keyword search across your codebase</p>
      </header>

      <form onSubmit={handleSearch} className="search-form">
        <input
          type="text"
          className="search-input"
          placeholder="Search code..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <button type="submit" className="search-button" disabled={loading}>
          {loading ? 'Searching...' : 'Search'}
        </button>
      </form>

      <div className="filters">
        <input
          type="text"
          className="filter-input"
          placeholder="Store (default)"
          value={store}
          onChange={(e) => setStore(e.target.value || 'default')}
        />
        <input
          type="text"
          className="filter-input"
          placeholder="Path prefix (e.g., src/)"
          value={pathPrefix}
          onChange={(e) => setPathPrefix(e.target.value)}
        />
      </div>

      {error && <div className="error">{error}</div>}

      {searchTime !== null && (
        <div className="results-info">
          Found {results.length} results in {searchTime}ms
        </div>
      )}

      {loading && <div className="loading">Searching...</div>}

      {!loading && results.length === 0 && searchTime !== null && (
        <div className="no-results">No results found</div>
      )}

      {results.map((result) => (
        <div key={result.doc_id} className="result-item">
          <div className="result-header">
            <span className="result-path">{result.path}</span>
            <div className="result-meta">
              <span>{result.language}</span>
              <span>
                L{result.start_line}-{result.end_line}
              </span>
              <span>Score: {result.final_score.toFixed(3)}</span>
            </div>
          </div>
          {result.symbols.length > 0 && (
            <div className="result-symbols">
              {result.symbols.map((sym, i) => (
                <span key={i}>{sym}</span>
              ))}
            </div>
          )}
          {result.content && (
            <div className="result-content">
              <pre>{result.content}</pre>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
