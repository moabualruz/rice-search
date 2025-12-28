'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { getStore, getStoreStats, getIndexStats, listFiles } from '@/lib/api';
import { ErrorBanner, LoadingSpinner } from '@/components';
import type { StoreInfo, StoreStats, IndexStats, TrackedFile, SortField, SortOrder } from '@/types';

// ============================================================================
// Utility Functions
// ============================================================================

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string): string => {
  if (!dateStr) return 'Never';
  const date = new Date(dateStr);
  return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
};

// Language options for filter
const LANGUAGES = [
  { value: '', label: 'All Languages' },
  { value: 'typescript', label: 'TypeScript' },
  { value: 'javascript', label: 'JavaScript' },
  { value: 'python', label: 'Python' },
  { value: 'rust', label: 'Rust' },
  { value: 'go', label: 'Go' },
  { value: 'java', label: 'Java' },
  { value: 'kotlin', label: 'Kotlin' },
  { value: 'csharp', label: 'C#' },
  { value: 'cpp', label: 'C++' },
  { value: 'c', label: 'C' },
  { value: 'ruby', label: 'Ruby' },
  { value: 'php', label: 'PHP' },
  { value: 'swift', label: 'Swift' },
  { value: 'scala', label: 'Scala' },
  { value: 'markdown', label: 'Markdown' },
  { value: 'json', label: 'JSON' },
  { value: 'yaml', label: 'YAML' },
];

// ============================================================================
// Store Details Page
// ============================================================================

export default function StoreDetailsPage() {
  const params = useParams();
  const storeName = params.store as string;

  // Store info
  const [storeInfo, setStoreInfo] = useState<StoreInfo | null>(null);
  const [storeStats, setStoreStats] = useState<StoreStats | null>(null);
  const [indexStats, setIndexStats] = useState<IndexStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Files table
  const [files, setFiles] = useState<TrackedFile[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(25);
  const [totalFiles, setTotalFiles] = useState(0);
  const [totalPages, setTotalPages] = useState(0);

  // Filters
  const [pathFilter, setPathFilter] = useState('');
  const [languageFilter, setLanguageFilter] = useState('');
  const [sortBy, setSortBy] = useState<SortField>('path');
  const [sortOrder, setSortOrder] = useState<SortOrder>('asc');

  // Debounced path filter
  const [debouncedPathFilter, setDebouncedPathFilter] = useState('');

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedPathFilter(pathFilter);
      setPage(1); // Reset to first page when filter changes
    }, 300);
    return () => clearTimeout(timer);
  }, [pathFilter]);

  // Fetch store details
  useEffect(() => {
    const fetchStoreDetails = async () => {
      setLoading(true);
      setError(null);
      try {
        const [info, stats, index] = await Promise.all([
          getStore(storeName),
          getStoreStats(storeName),
          getIndexStats(storeName),
        ]);
        setStoreInfo(info);
        setStoreStats(stats);
        setIndexStats(index);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load store details');
      } finally {
        setLoading(false);
      }
    };
    fetchStoreDetails();
  }, [storeName]);

  // Fetch files
  const fetchFiles = useCallback(async () => {
    setFilesLoading(true);
    try {
      const response = await listFiles(storeName, {
        page,
        pageSize,
        pathFilter: debouncedPathFilter || undefined,
        language: languageFilter || undefined,
        sortBy,
        sortOrder,
      });
      setFiles(response.files);
      setTotalFiles(response.total);
      setTotalPages(response.total_pages);
    } catch (err) {
      console.error('Failed to load files:', err);
    } finally {
      setFilesLoading(false);
    }
  }, [storeName, page, pageSize, debouncedPathFilter, languageFilter, sortBy, sortOrder]);

  useEffect(() => {
    fetchFiles();
  }, [fetchFiles]);

  // Sort handler
  const handleSort = (field: SortField) => {
    if (sortBy === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setSortOrder('asc');
    }
    setPage(1);
  };

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortBy !== field) return <span className="sort-icon">↕️</span>;
    return <span className="sort-icon">{sortOrder === 'asc' ? '↑' : '↓'}</span>;
  };

  if (loading) {
    return (
      <main className="main admin-page">
        <LoadingSpinner size="large" message="Loading store details..." />
      </main>
    );
  }

  if (error || !storeInfo) {
    return (
      <main className="main admin-page">
        <div className="admin-header">
          <Link href="/admin" className="back-link">
            ← Back to Stores
          </Link>
          <h1>Store Not Found</h1>
        </div>
        {error && <ErrorBanner message={error} />}
      </main>
    );
  }

  return (
    <main className="main admin-page">
      {/* Header */}
      <div className="admin-header">
        <Link href="/admin" className="back-link">
          ← Back to Stores
        </Link>
        <h1>📦 {storeName}</h1>
        {storeInfo.description && <p className="admin-subtitle">{storeInfo.description}</p>}
      </div>

      {/* Stats Cards */}
      <div className="stats-cards">
        <div className="stat-card">
          <div className="stat-card-value">{storeStats?.sparse_index?.doc_count ?? 0}</div>
          <div className="stat-card-label">BM25 Documents</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-value">{storeStats?.dense_index?.doc_count ?? 0}</div>
          <div className="stat-card-label">Vector Documents</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-value">{indexStats?.tracked_files ?? 0}</div>
          <div className="stat-card-label">Tracked Files</div>
        </div>
        <div className="stat-card">
          <div className="stat-card-value">{formatBytes(indexStats?.total_size ?? 0)}</div>
          <div className="stat-card-label">Total Size</div>
        </div>
      </div>

      {/* Index Details */}
      <div className="admin-card">
        <h2 className="admin-card-title">📊 Index Statistics</h2>
        <div className="details-grid">
          <div className="detail-row">
            <span className="detail-label">Sparse Segments:</span>
            <span className="detail-value">{storeStats?.sparse_index?.segment_count ?? 0}</span>
          </div>
          <div className="detail-row">
            <span className="detail-label">Dense Index:</span>
            <span className="detail-value">{storeStats?.dense_index?.exists ? 'Active' : 'Not Created'}</span>
          </div>
          <div className="detail-row">
            <span className="detail-label">Created:</span>
            <span className="detail-value">{formatDate(storeInfo.created_at)}</span>
          </div>
          <div className="detail-row">
            <span className="detail-label">Last Updated:</span>
            <span className="detail-value">{formatDate(indexStats?.last_updated ?? '')}</span>
          </div>
        </div>
      </div>

      {/* Files Table */}
      <div className="admin-card">
        <h2 className="admin-card-title">📁 Indexed Files</h2>

        {/* Filters */}
        <div className="table-filters">
          <div className="filter-group">
            <label className="filter-label">Search Path</label>
            <input
              type="text"
              className="filter-input"
              placeholder="Filter by path..."
              value={pathFilter}
              onChange={(e) => setPathFilter(e.target.value)}
            />
          </div>
          <div className="filter-group">
            <label className="filter-label">Language</label>
            <select
              className="filter-input"
              value={languageFilter}
              onChange={(e) => {
                setLanguageFilter(e.target.value);
                setPage(1);
              }}
            >
              {LANGUAGES.map((lang) => (
                <option key={lang.value} value={lang.value}>
                  {lang.label}
                </option>
              ))}
            </select>
          </div>
          <div className="filter-info">
            Showing {files.length} of {totalFiles} files
          </div>
        </div>

        {/* Table */}
        {filesLoading && files.length === 0 ? (
          <LoadingSpinner message="Loading files..." />
        ) : files.length === 0 ? (
          <div className="empty-state">
            <span className="empty-icon">📭</span>
            <p>No files match your filters</p>
          </div>
        ) : (
          <>
            <div className="files-table-wrapper">
              <table className="files-table">
                <thead>
                  <tr>
                    <th onClick={() => handleSort('path')} className="sortable">
                      Path <SortIcon field="path" />
                    </th>
                    <th>Language</th>
                    <th onClick={() => handleSort('size')} className="sortable">
                      Size <SortIcon field="size" />
                    </th>
                    <th>Chunks</th>
                    <th onClick={() => handleSort('indexed_at')} className="sortable">
                      Indexed <SortIcon field="indexed_at" />
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {files.map((file) => (
                    <tr key={file.path}>
                      <td className="file-path-cell" title={file.path}>
                        <span className="file-path">{file.path}</span>
                      </td>
                      <td>
                        <span className="language-badge">{file.language || 'unknown'}</span>
                      </td>
                      <td>{formatBytes(file.size)}</td>
                      <td>{file.chunk_count}</td>
                      <td className="text-muted">{formatDate(file.indexed_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="pagination">
                <button
                  className="pagination-btn"
                  onClick={() => setPage(1)}
                  disabled={page === 1}
                >
                  ««
                </button>
                <button
                  className="pagination-btn"
                  onClick={() => setPage(page - 1)}
                  disabled={page === 1}
                >
                  «
                </button>
                <span className="pagination-info">
                  Page {page} of {totalPages}
                </span>
                <button
                  className="pagination-btn"
                  onClick={() => setPage(page + 1)}
                  disabled={page === totalPages}
                >
                  »
                </button>
                <button
                  className="pagination-btn"
                  onClick={() => setPage(totalPages)}
                  disabled={page === totalPages}
                >
                  »»
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </main>
  );
}
