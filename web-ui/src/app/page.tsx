'use client';

import { useState, FormEvent } from 'react';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8088';

// ============================================================================
// Type Definitions (Phases 0-5)
// ============================================================================

interface SearchFilters {
  path_prefix?: string;
  languages?: string[];
}

interface SearchRequest {
  query: string;
  top_k?: number;
  filters?: SearchFilters;
  include_content?: boolean;
  // Phase 1: Reranking options
  enable_reranking?: boolean;
  rerank_candidates?: number;
  sparse_weight?: number;
  dense_weight?: number;
  // Phase 2: PostRank options
  enable_dedup?: boolean;
  dedup_threshold?: number;
  enable_diversity?: boolean;
  diversity_lambda?: number;
  group_by_file?: boolean;
  max_chunks_per_file?: number;
  // Phase 5: Query expansion
  enable_expansion?: boolean;
}

interface AggregationInfo {
  is_representative: boolean;
  related_chunks: number;
  file_score: number;
  chunk_rank_in_file: number;
}

interface SearchResult {
  doc_id: string;
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content?: string;
  symbols: string[];
  final_score: number;
  sparse_score?: number;
  dense_score?: number;
  sparse_rank?: number;
  dense_rank?: number;
  aggregation?: AggregationInfo;
}

interface IntelligenceInfo {
  intent: 'navigational' | 'factual' | 'exploratory' | 'analytical';
  difficulty: 'easy' | 'medium' | 'hard';
  strategy: 'sparse-only' | 'balanced' | 'dense-heavy' | 'deep-rerank';
  confidence: number;
}

interface RerankingInfo {
  enabled: boolean;
  candidates: number;
  pass1_applied: boolean;
  pass1_latency_ms: number;
  pass2_applied: boolean;
  pass2_latency_ms: number;
  early_exit: boolean;
  early_exit_reason?: string;
}

interface PostrankInfo {
  dedup: {
    input_count: number;
    output_count: number;
    removed: number;
    latency_ms: number;
  };
  diversity: {
    enabled: boolean;
    avg_diversity: number;
    latency_ms: number;
  };
  aggregation: {
    unique_files: number;
    chunks_dropped: number;
  };
  total_latency_ms: number;
}

interface SearchResponse {
  query: string;
  results: SearchResult[];
  total: number;
  store: string;
  search_time_ms: number;
  intelligence?: IntelligenceInfo;
  reranking?: RerankingInfo;
  postrank?: PostrankInfo;
}

// ============================================================================
// Constants
// ============================================================================

const services = [
  {
    name: 'API Docs',
    description: 'Swagger API Documentation',
    url: `${API_URL}/docs`,
    icon: '📚',
  },
  {
    name: 'Metrics',
    description: 'Observability Dashboard',
    url: `${API_URL}/v1/observability/stats`,
    icon: '📊',
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

const intentIcons: Record<string, string> = {
  navigational: '🎯',
  factual: '❓',
  exploratory: '🔍',
  analytical: '📊',
};

const strategyColors: Record<string, string> = {
  'sparse-only': 'var(--accent-orange)',
  'balanced': 'var(--accent-blue)',
  'dense-heavy': 'var(--accent-purple)',
  'deep-rerank': 'var(--accent-green)',
};

const difficultyColors: Record<string, string> = {
  easy: 'var(--accent-green)',
  medium: 'var(--accent-orange)',
  hard: 'var(--accent-red)',
};

// ============================================================================
// Main Component
// ============================================================================

export default function Home() {
  // Basic search state
  const [query, setQuery] = useState('');
  const [pathPrefix, setPathPrefix] = useState('');
  const [store, setStore] = useState('default');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searchTime, setSearchTime] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Response metadata
  const [intelligence, setIntelligence] = useState<IntelligenceInfo | null>(null);
  const [reranking, setReranking] = useState<RerankingInfo | null>(null);
  const [postrank, setPostrank] = useState<PostrankInfo | null>(null);

  // Advanced options (collapsed by default)
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Phase 1 options
  const [enableReranking, setEnableReranking] = useState(true);
  const [sparseWeight, setSparseWeight] = useState(50);

  // Phase 2 options
  const [enableDedup, setEnableDedup] = useState(true);
  const [dedupThreshold, setDedupThreshold] = useState(85);
  const [enableDiversity, setEnableDiversity] = useState(true);
  const [diversityLambda, setDiversityLambda] = useState(70);
  const [groupByFile, setGroupByFile] = useState(false);
  const [maxChunksPerFile, setMaxChunksPerFile] = useState(3);

  // Phase 5 options
  const [enableExpansion, setEnableExpansion] = useState(true);

  const handleSearch = async (e: FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setError(null);
    setIntelligence(null);
    setReranking(null);
    setPostrank(null);

    try {
      const requestBody: SearchRequest = {
        query: query.trim(),
        top_k: 20,
        include_content: true,
        filters: pathPrefix ? { path_prefix: pathPrefix } : undefined,
        // Phase 1
        enable_reranking: enableReranking,
        sparse_weight: sparseWeight / 100,
        dense_weight: (100 - sparseWeight) / 100,
        // Phase 2
        enable_dedup: enableDedup,
        dedup_threshold: dedupThreshold / 100,
        enable_diversity: enableDiversity,
        diversity_lambda: diversityLambda / 100,
        group_by_file: groupByFile,
        max_chunks_per_file: maxChunksPerFile,
        // Phase 5
        enable_expansion: enableExpansion,
      };

      const response = await fetch('/api/v1/stores/' + store + '/search', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(requestBody),
      });

      if (!response.ok) {
        throw new Error(`Search failed: ${response.statusText}`);
      }

      const data: SearchResponse = await response.json();
      setResults(data.results);
      setSearchTime(data.search_time_ms);
      setIntelligence(data.intelligence || null);
      setReranking(data.reranking || null);
      setPostrank(data.postrank || null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className='app'>
      {/* Navigation */}
      <nav className='navbar'>
        <div className='nav-brand'>
          <span className='brand-icon'>🍚</span>
          <span className='brand-text'>Rice Search</span>
        </div>
        <div className='nav-links'>
          {services.map((service) => (
            <a
              key={service.name}
              href={service.url}
              target='_blank'
              rel='noopener noreferrer'
              className='nav-link'
              title={service.description}
            >
              <span className='nav-icon'>{service.icon}</span>
              <span className='nav-label'>{service.name}</span>
            </a>
          ))}
        </div>
      </nav>

      {/* Hero Section */}
      <header className='hero'>
        <h1 className='hero-title'>
          <span className='hero-icon'>🍚</span>
          Rice Search
        </h1>
        <p className='hero-subtitle'>
          Intelligent hybrid semantic + keyword search across your codebase
        </p>
        <div className='hero-badges'>
          <span className='badge'>BM25</span>
          <span className='badge-plus'>+</span>
          <span className='badge'>Semantic</span>
          <span className='badge-plus'>+</span>
          <span className='badge'>Reranking</span>
          <span className='badge-plus'>=</span>
          <span className='badge badge-highlight'>Intelligent Search</span>
        </div>
      </header>

      {/* Main Content */}
      <main className='main'>
        {/* Search Section */}
        <section className='search-section'>
          <form onSubmit={handleSearch} className='search-form'>
            <div className='search-input-wrapper'>
              <span className='search-icon'>🔍</span>
              <input
                type='text'
                className='search-input'
                placeholder="Search your codebase... (e.g., 'authentication handler', 'config loading')"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <button type='submit' className='search-button' disabled={loading}>
              {loading ? (
                <span className='loading-spinner'></span>
              ) : (
                'Search'
              )}
            </button>
          </form>

          {/* Basic Filters */}
          <div className='filters'>
            <div className='filter-group'>
              <label className='filter-label'>Store</label>
              <input
                type='text'
                className='filter-input'
                placeholder='default'
                value={store}
                onChange={(e) => setStore(e.target.value || 'default')}
              />
            </div>
            <div className='filter-group'>
              <label className='filter-label'>Path Filter</label>
              <input
                type='text'
                className='filter-input'
                placeholder='e.g., src/components/'
                value={pathPrefix}
                onChange={(e) => setPathPrefix(e.target.value)}
              />
            </div>
            <button
              type='button'
              className='advanced-toggle'
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? '▲ Hide' : '▼ Advanced'} Options
            </button>
          </div>

          {/* Advanced Options Panel */}
          {showAdvanced && (
            <div className='advanced-panel'>
              <div className='advanced-section'>
                <h4 className='advanced-title'>🎯 Retrieval</h4>
                <div className='advanced-grid'>
                  <label className='checkbox-label'>
                    <input
                      type='checkbox'
                      checked={enableReranking}
                      onChange={(e) => setEnableReranking(e.target.checked)}
                    />
                    Enable Reranking
                  </label>
                  <label className='checkbox-label'>
                    <input
                      type='checkbox'
                      checked={enableExpansion}
                      onChange={(e) => setEnableExpansion(e.target.checked)}
                    />
                    Query Expansion
                  </label>
                  <div className='slider-group'>
                    <label className='slider-label'>
                      Sparse Weight: {sparseWeight}%
                    </label>
                    <input
                      type='range'
                      min='0'
                      max='100'
                      value={sparseWeight}
                      onChange={(e) => setSparseWeight(Number(e.target.value))}
                      className='slider'
                    />
                    <span className='slider-hint'>
                      Dense: {100 - sparseWeight}%
                    </span>
                  </div>
                </div>
              </div>

              <div className='advanced-section'>
                <h4 className='advanced-title'>✨ Post-Processing</h4>
                <div className='advanced-grid'>
                  <label className='checkbox-label'>
                    <input
                      type='checkbox'
                      checked={enableDedup}
                      onChange={(e) => setEnableDedup(e.target.checked)}
                    />
                    Deduplication
                  </label>
                  <label className='checkbox-label'>
                    <input
                      type='checkbox'
                      checked={enableDiversity}
                      onChange={(e) => setEnableDiversity(e.target.checked)}
                    />
                    Diversity (MMR)
                  </label>
                  <label className='checkbox-label'>
                    <input
                      type='checkbox'
                      checked={groupByFile}
                      onChange={(e) => setGroupByFile(e.target.checked)}
                    />
                    Group by File
                  </label>
                  {enableDedup && (
                    <div className='slider-group'>
                      <label className='slider-label'>
                        Dedup Threshold: {dedupThreshold}%
                      </label>
                      <input
                        type='range'
                        min='50'
                        max='100'
                        value={dedupThreshold}
                        onChange={(e) => setDedupThreshold(Number(e.target.value))}
                        className='slider'
                      />
                    </div>
                  )}
                  {enableDiversity && (
                    <div className='slider-group'>
                      <label className='slider-label'>
                        Diversity λ: {diversityLambda}%
                      </label>
                      <input
                        type='range'
                        min='0'
                        max='100'
                        value={diversityLambda}
                        onChange={(e) => setDiversityLambda(Number(e.target.value))}
                        className='slider'
                      />
                      <span className='slider-hint'>
                        0=diverse, 100=relevant
                      </span>
                    </div>
                  )}
                  {groupByFile && (
                    <div className='number-group'>
                      <label className='slider-label'>Max Chunks/File:</label>
                      <input
                        type='number'
                        min='1'
                        max='10'
                        value={maxChunksPerFile}
                        onChange={(e) => setMaxChunksPerFile(Number(e.target.value))}
                        className='number-input'
                      />
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}
        </section>

        {/* Error Display */}
        {error && (
          <div className='error-banner'>
            <span className='error-icon'>⚠️</span>
            {error}
          </div>
        )}

        {/* Intelligence Banner */}
        {intelligence && (
          <div className='intelligence-banner'>
            <div className='intel-item'>
              <span className='intel-icon'>{intentIcons[intelligence.intent]}</span>
              <span className='intel-label'>Intent:</span>
              <span className='intel-value'>{intelligence.intent}</span>
            </div>
            <div className='intel-item'>
              <span className='intel-label'>Strategy:</span>
              <span
                className='intel-badge'
                style={{ backgroundColor: strategyColors[intelligence.strategy] }}
              >
                {intelligence.strategy}
              </span>
            </div>
            <div className='intel-item'>
              <span className='intel-label'>Difficulty:</span>
              <span
                className='intel-dot'
                style={{ backgroundColor: difficultyColors[intelligence.difficulty] }}
              ></span>
              <span className='intel-value'>{intelligence.difficulty}</span>
            </div>
            <div className='intel-item'>
              <span className='intel-label'>Confidence:</span>
              <span className='intel-value'>
                {(intelligence.confidence * 100).toFixed(0)}%
              </span>
            </div>
          </div>
        )}

        {/* Stats Panel */}
        {searchTime !== null && (
          <div className='stats-panel'>
            <div className='stat-item'>
              <span className='stat-value'>{results.length}</span>
              <span className='stat-label'>results</span>
            </div>
            <div className='stat-item'>
              <span className='stat-value'>{searchTime}ms</span>
              <span className='stat-label'>total</span>
            </div>
            {reranking && reranking.enabled && (
              <div className='stat-item'>
                <span className='stat-value'>
                  {reranking.pass1_latency_ms + reranking.pass2_latency_ms}ms
                </span>
                <span className='stat-label'>rerank</span>
                {reranking.early_exit && (
                  <span className='stat-badge' title={reranking.early_exit_reason}>
                    ⚡ early
                  </span>
                )}
              </div>
            )}
            {postrank && (
              <>
                <div className='stat-item'>
                  <span className='stat-value'>{postrank.total_latency_ms}ms</span>
                  <span className='stat-label'>postrank</span>
                </div>
                {postrank.dedup.removed > 0 && (
                  <div className='stat-item'>
                    <span className='stat-value'>-{postrank.dedup.removed}</span>
                    <span className='stat-label'>deduped</span>
                  </div>
                )}
                {postrank.diversity.enabled && (
                  <div className='stat-item'>
                    <span className='stat-value'>
                      {(postrank.diversity.avg_diversity * 100).toFixed(0)}%
                    </span>
                    <span className='stat-label'>diversity</span>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* Results Section */}
        <section className='results-section'>
          {loading && (
            <div className='loading-state'>
              <div className='loading-spinner large'></div>
              <p>Searching across your codebase...</p>
            </div>
          )}

          {!loading && results.length === 0 && searchTime !== null && (
            <div className='empty-state'>
              <span className='empty-icon'>📭</span>
              <h3>No results found</h3>
              <p>Try adjusting your search query or filters</p>
            </div>
          )}

          {!loading && results.length === 0 && searchTime === null && (
            <div className='welcome-state'>
              <div className='services-grid'>
                {services.map((service) => (
                  <a
                    key={service.name}
                    href={service.url}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='service-card'
                  >
                    <span className='service-icon'>{service.icon}</span>
                    <h3 className='service-name'>{service.name}</h3>
                    <p className='service-desc'>{service.description}</p>
                  </a>
                ))}
              </div>
              <div className='tips'>
                <h4>🧠 Intelligent Search Features</h4>
                <ul>
                  <li>
                    <strong>Intent Detection:</strong> Automatically detects if you&apos;re navigating, exploring, or analyzing
                  </li>
                  <li>
                    <strong>Query Expansion:</strong> Expands &quot;auth&quot; to &quot;authentication, authorization&quot;
                  </li>
                  <li>
                    <strong>Hybrid Search:</strong> Combines keyword (BM25) + semantic for best results
                  </li>
                  <li>
                    <strong>Neural Reranking:</strong> AI-powered result ordering
                  </li>
                  <li>
                    <strong>Diversity:</strong> Avoids showing duplicate/similar results
                  </li>
                </ul>
              </div>
            </div>
          )}

          <div className='results-list'>
            {results.map((result) => (
              <article key={result.doc_id} className='result-card'>
                <header className='result-header'>
                  <div className='result-file'>
                    <span className='file-icon'>📄</span>
                    <span className='file-path'>{result.path}</span>
                    {result.aggregation?.is_representative && (
                      <span className='rep-badge' title='Representative chunk for this file'>
                        ★ Rep
                      </span>
                    )}
                  </div>
                  <div className='result-meta'>
                    <span className='meta-lang'>{result.language}</span>
                    <span className='meta-lines'>
                      L{result.start_line}-{result.end_line}
                    </span>
                    <span className='meta-score'>
                      {(result.final_score * 100).toFixed(1)}%
                    </span>
                  </div>
                </header>

                {/* Score Details */}
                <div className='score-details'>
                  {result.sparse_score !== undefined && (
                    <span className='score-badge sparse'>
                      BM25: {result.sparse_score.toFixed(2)}
                      {result.sparse_rank && <small> (#{result.sparse_rank})</small>}
                    </span>
                  )}
                  {result.dense_score !== undefined && (
                    <span className='score-badge dense'>
                      Semantic: {result.dense_score.toFixed(2)}
                      {result.dense_rank && <small> (#{result.dense_rank})</small>}
                    </span>
                  )}
                  {result.aggregation && (
                    <span className='score-badge agg'>
                      File: {(result.aggregation.file_score * 100).toFixed(0)}%
                      {result.aggregation.related_chunks > 0 && (
                        <small> +{result.aggregation.related_chunks} chunks</small>
                      )}
                    </span>
                  )}
                </div>

                {result.symbols.length > 0 && (
                  <div className='result-symbols'>
                    {result.symbols.slice(0, 8).map((sym, i) => (
                      <span key={i} className='symbol-tag'>
                        {sym}
                      </span>
                    ))}
                    {result.symbols.length > 8 && (
                      <span className='symbol-more'>
                        +{result.symbols.length - 8} more
                      </span>
                    )}
                  </div>
                )}

                {result.content && (
                  <div className='result-code'>
                    <div className='code-header'>
                      <span className='code-lang'>{result.language}</span>
                      <span className='code-lines'>
                        lines {result.start_line}-{result.end_line}
                      </span>
                    </div>
                    <pre className='code-content'>
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
      <footer className='footer'>
        <p>
          Rice Search Platform • Intelligent Hybrid Code Search
        </p>
        <p className='footer-links'>
          <a href={`${API_URL}/docs`} target='_blank' rel='noopener noreferrer'>
            API Docs
          </a>
          <span className='footer-sep'>•</span>
          <a href={`${API_URL}/metrics`} target='_blank' rel='noopener noreferrer'>
            Metrics
          </a>
          <span className='footer-sep'>•</span>
          <a href='https://github.com' target='_blank' rel='noopener noreferrer'>
            GitHub
          </a>
        </p>
      </footer>
    </div>
  );
}
