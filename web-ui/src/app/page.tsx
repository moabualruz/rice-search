'use client';

import { useState, FormEvent } from 'react';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8088';

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

const services = [
  {
    name: 'API Docs',
    description: 'Swagger API Documentation',
    url: `${API_URL}/docs`,
    icon: '📚',
  },
  {
    name: 'Attu',
    description: 'Milvus Vector DB Admin',
    url: 'http://localhost:8000',
    icon: '🗄️',
  },
  {
    name: 'MinIO',
    description: 'Object Storage Console',
    url: 'http://localhost:9001',
    icon: '💾',
  },
  {
    name: 'Health',
    description: 'API Health Status',
    url: `${API_URL}/healthz`,
    icon: '💚',
  },
];

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
      const response = await fetch('/api/v1/stores/' + store + '/search', {
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
    <div className="app">
      {/* Navigation */}
      <nav className="navbar">
        <div className="nav-brand">
          <span className="brand-icon">🍚</span>
          <span className="brand-text">Rice Search</span>
        </div>
        <div className="nav-links">
          {services.map((service) => (
            <a
              key={service.name}
              href={service.url}
              target="_blank"
              rel="noopener noreferrer"
              className="nav-link"
              title={service.description}
            >
              <span className="nav-icon">{service.icon}</span>
              <span className="nav-label">{service.name}</span>
            </a>
          ))}
        </div>
      </nav>

      {/* Hero Section */}
      <header className="hero">
        <h1 className="hero-title">
          <span className="hero-icon">🍚</span>
          Rice Search
        </h1>
        <p className="hero-subtitle">
          Hybrid semantic + keyword search across your codebase
        </p>
        <div className="hero-badges">
          <span className="badge">BM25</span>
          <span className="badge-plus">+</span>
          <span className="badge">Semantic</span>
          <span className="badge-plus">=</span>
          <span className="badge badge-highlight">Hybrid Search</span>
        </div>
      </header>

      {/* Main Content */}
      <main className="main">
        {/* Search Section */}
        <section className="search-section">
          <form onSubmit={handleSearch} className="search-form">
            <div className="search-input-wrapper">
              <span className="search-icon">🔍</span>
              <input
                type="text"
                className="search-input"
                placeholder="Search your codebase... (e.g., 'authentication handler', 'config loading')"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <button type="submit" className="search-button" disabled={loading}>
              {loading ? (
                <span className="loading-spinner"></span>
              ) : (
                'Search'
              )}
            </button>
          </form>

          <div className="filters">
            <div className="filter-group">
              <label className="filter-label">Store</label>
              <input
                type="text"
                className="filter-input"
                placeholder="default"
                value={store}
                onChange={(e) => setStore(e.target.value || 'default')}
              />
            </div>
            <div className="filter-group">
              <label className="filter-label">Path Filter</label>
              <input
                type="text"
                className="filter-input"
                placeholder="e.g., src/components/"
                value={pathPrefix}
                onChange={(e) => setPathPrefix(e.target.value)}
              />
            </div>
          </div>
        </section>

        {/* Error Display */}
        {error && (
          <div className="error-banner">
            <span className="error-icon">⚠️</span>
            {error}
          </div>
        )}

        {/* Results Section */}
        <section className="results-section">
          {searchTime !== null && (
            <div className="results-header">
              <span className="results-count">
                {results.length} result{results.length !== 1 ? 's' : ''}
              </span>
              <span className="results-time">{searchTime}ms</span>
            </div>
          )}

          {loading && (
            <div className="loading-state">
              <div className="loading-spinner large"></div>
              <p>Searching across your codebase...</p>
            </div>
          )}

          {!loading && results.length === 0 && searchTime !== null && (
            <div className="empty-state">
              <span className="empty-icon">📭</span>
              <h3>No results found</h3>
              <p>Try adjusting your search query or filters</p>
            </div>
          )}

          {!loading && results.length === 0 && searchTime === null && (
            <div className="welcome-state">
              <div className="services-grid">
                {services.map((service) => (
                  <a
                    key={service.name}
                    href={service.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="service-card"
                  >
                    <span className="service-icon">{service.icon}</span>
                    <h3 className="service-name">{service.name}</h3>
                    <p className="service-desc">{service.description}</p>
                  </a>
                ))}
              </div>
              <div className="tips">
                <h4>Search Tips</h4>
                <ul>
                  <li>Use natural language: &quot;where do we handle authentication&quot;</li>
                  <li>Search for patterns: &quot;async function that fetches data&quot;</li>
                  <li>Find by concept: &quot;error handling middleware&quot;</li>
                </ul>
              </div>
            </div>
          )}

          <div className="results-list">
            {results.map((result) => (
              <article key={result.doc_id} className="result-card">
                <header className="result-header">
                  <div className="result-file">
                    <span className="file-icon">📄</span>
                    <span className="file-path">{result.path}</span>
                  </div>
                  <div className="result-meta">
                    <span className="meta-lang">{result.language}</span>
                    <span className="meta-lines">
                      L{result.start_line}-{result.end_line}
                    </span>
                    <span className="meta-score">
                      {(result.final_score * 100).toFixed(1)}%
                    </span>
                  </div>
                </header>

                {result.symbols.length > 0 && (
                  <div className="result-symbols">
                    {result.symbols.slice(0, 8).map((sym, i) => (
                      <span key={i} className="symbol-tag">
                        {sym}
                      </span>
                    ))}
                    {result.symbols.length > 8 && (
                      <span className="symbol-more">
                        +{result.symbols.length - 8} more
                      </span>
                    )}
                  </div>
                )}

                {result.content && (
                  <div className="result-code">
                    <div className="code-header">
                      <span className="code-lang">{result.language}</span>
                      <span className="code-lines">
                        lines {result.start_line}-{result.end_line}
                      </span>
                    </div>
                    <pre className="code-content">
                      <code>{result.content}</code>
                    </pre>
                  </div>
                )}
              </article>
            ))}
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="footer">
        <p>
          Rice Search Platform • Hybrid BM25 + Semantic Code Search
        </p>
        <p className="footer-links">
          <a href={`${API_URL}/docs`} target="_blank" rel="noopener noreferrer">
            API Docs
          </a>
          <span className="footer-sep">•</span>
          <a href="https://github.com" target="_blank" rel="noopener noreferrer">
            GitHub
          </a>
        </p>
      </footer>
    </div>
  );
}
