'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import {
  getObservabilityStats,
  getTelemetryRecords,
  listStores,
  getQueryStats,
  getRecentQueries,
} from '@/lib/api';
import { ErrorBanner, LoadingSpinner, ConnectionWarning } from '@/components';
import type {
  ObservabilityStats,
  TelemetryRecord,
  StoreInfo,
  QueryStats,
  QueryLogEntry,
} from '@/types';

// ============================================================================
// Utility Functions
// ============================================================================

const formatNumber = (num: number, decimals = 2): string => {
  if (num === 0) return '0';
  if (num >= 1000000) return (num / 1000000).toFixed(decimals) + 'M';
  if (num >= 1000) return (num / 1000).toFixed(decimals) + 'K';
  return num.toFixed(decimals);
};

const formatPercent = (num: number): string => {
  return (num * 100).toFixed(1) + '%';
};

const formatMs = (ms: number): string => {
  if (ms < 1) return '<1ms';
  if (ms >= 1000) return (ms / 1000).toFixed(2) + 's';
  return ms.toFixed(0) + 'ms';
};

const formatDate = (dateStr: string): string => {
  const date = new Date(dateStr);
  return date.toLocaleTimeString();
};

// ============================================================================
// Stat Card Component
// ============================================================================

interface StatCardProps {
  label: string;
  value: string | number;
  subtitle?: string;
  trend?: 'up' | 'down' | 'neutral';
  icon?: string;
}

function StatCard({ label, value, subtitle, icon }: StatCardProps) {
  return (
    <div className="stat-card">
      <div className="stat-card-header">
        {icon && <span className="stat-card-icon">{icon}</span>}
        <span className="stat-card-label">{label}</span>
      </div>
      <div className="stat-card-value">{value}</div>
      {subtitle && <div className="stat-card-subtitle">{subtitle}</div>}
    </div>
  );
}

// ============================================================================
// Distribution Chart Component
// ============================================================================

interface DistributionChartProps {
  title: string;
  data: Record<string, number>;
  colorScheme?: 'blue' | 'green' | 'purple';
}

function DistributionChart({ title, data, colorScheme = 'blue' }: DistributionChartProps) {
  const total = Object.values(data).reduce((a, b) => a + b, 0);
  const entries = Object.entries(data).sort((a, b) => b[1] - a[1]);

  const colors: Record<string, string[]> = {
    blue: ['#3b82f6', '#60a5fa', '#93c5fd', '#bfdbfe', '#dbeafe'],
    green: ['#10b981', '#34d399', '#6ee7b7', '#a7f3d0', '#d1fae5'],
    purple: ['#8b5cf6', '#a78bfa', '#c4b5fd', '#ddd6fe', '#ede9fe'],
  };

  return (
    <div className="distribution-chart">
      <h3 className="distribution-title">{title}</h3>
      <div className="distribution-bars">
        {entries.map(([key, value], index) => {
          const percent = total > 0 ? (value / total) * 100 : 0;
          const color = colors[colorScheme][index % colors[colorScheme].length];
          return (
            <div key={key} className="distribution-item">
              <div className="distribution-label">
                <span className="distribution-key">{key}</span>
                <span className="distribution-value">{value} ({percent.toFixed(1)}%)</span>
              </div>
              <div className="distribution-bar-bg">
                <div
                  className="distribution-bar"
                  style={{ width: `${percent}%`, backgroundColor: color }}
                />
              </div>
            </div>
          );
        })}
        {entries.length === 0 && (
          <div className="empty-state-small">No data available</div>
        )}
      </div>
    </div>
  );
}

// ============================================================================
// Recent Queries Table Component
// ============================================================================

interface RecentQueriesTableProps {
  queries: QueryLogEntry[];
  loading: boolean;
}

function RecentQueriesTable({ queries, loading }: RecentQueriesTableProps) {
  if (loading) {
    return <LoadingSpinner message="Loading queries..." />;
  }

  if (queries.length === 0) {
    return (
      <div className="empty-state-small">
        <span>No recent queries</span>
      </div>
    );
  }

  return (
    <div className="queries-table-wrapper">
      <table className="queries-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Query</th>
            <th>Intent</th>
            <th>Strategy</th>
            <th>Results</th>
            <th>Latency</th>
          </tr>
        </thead>
        <tbody>
          {queries.slice(0, 20).map((q, i) => (
            <tr key={i}>
              <td className="text-muted">{formatDate(q.timestamp)}</td>
              <td className="query-cell" title={q.query}>
                {q.query.substring(0, 50)}{q.query.length > 50 ? '...' : ''}
              </td>
              <td>
                <span className={`badge badge-${q.intent}`}>{q.intent}</span>
              </td>
              <td>
                <span className={`badge badge-strategy-${q.strategy}`}>{q.strategy}</span>
              </td>
              <td>{q.resultCount}</td>
              <td className={q.latencyMs > 100 ? 'text-warning' : ''}>{formatMs(q.latencyMs)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ============================================================================
// Telemetry Table Component
// ============================================================================

interface TelemetryTableProps {
  records: TelemetryRecord[];
  loading: boolean;
}

function TelemetryTable({ records, loading }: TelemetryTableProps) {
  if (loading) {
    return <LoadingSpinner message="Loading telemetry..." />;
  }

  if (records.length === 0) {
    return (
      <div className="empty-state-small">
        <span>No telemetry records</span>
      </div>
    );
  }

  return (
    <div className="telemetry-table-wrapper">
      <table className="telemetry-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Store</th>
            <th>Query</th>
            <th>Sparse</th>
            <th>Dense</th>
            <th>Rerank</th>
            <th>Total</th>
          </tr>
        </thead>
        <tbody>
          {records.slice(0, 20).map((r) => (
            <tr key={r.requestId}>
              <td className="text-muted">{formatDate(r.timestamp)}</td>
              <td>{r.store}</td>
              <td className="query-cell" title={r.query}>
                {r.query.substring(0, 40)}{r.query.length > 40 ? '...' : ''}
              </td>
              <td>{formatMs(r.sparse.latencyMs)}</td>
              <td>{formatMs(r.dense.latencyMs)}</td>
              <td>
                {r.rerank.enabled ? (
                  r.rerank.skipped ? (
                    <span className="text-muted">skipped</span>
                  ) : (
                    formatMs(r.rerank.latencyMs)
                  )
                ) : (
                  <span className="text-muted">off</span>
                )}
              </td>
              <td className={r.totalLatencyMs > 100 ? 'text-warning' : 'text-success'}>
                {formatMs(r.totalLatencyMs)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ============================================================================
// Main Observability Page
// ============================================================================

export default function ObservabilityPage() {
  const [stats, setStats] = useState<ObservabilityStats | null>(null);
  const [telemetry, setTelemetry] = useState<TelemetryRecord[]>([]);
  const [stores, setStores] = useState<StoreInfo[]>([]);
  const [selectedStore, setSelectedStore] = useState<string>('');
  const [queryStats, setQueryStats] = useState<QueryStats | null>(null);
  const [recentQueries, setRecentQueries] = useState<QueryLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [queriesLoading, setQueriesLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch global stats and telemetry
  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      setError(null);
      try {
        console.log('[Observability] Fetching data...');
        
        // Fetch in parallel but handle each separately
        const [statsResult, telemetryResult, storesResult] = await Promise.allSettled([
          getObservabilityStats(),
          getTelemetryRecords(50),
          listStores(),
        ]);

        // Handle stats
        if (statsResult.status === 'fulfilled') {
          console.log('[Observability] Stats loaded:', statsResult.value);
          setStats(statsResult.value);
        } else {
          console.error('[Observability] Failed to load stats:', statsResult.reason);
        }

        // Handle telemetry
        if (telemetryResult.status === 'fulfilled') {
          console.log('[Observability] Telemetry loaded:', telemetryResult.value);
          setTelemetry(telemetryResult.value.records || []);
        } else {
          console.error('[Observability] Failed to load telemetry:', telemetryResult.reason);
        }

        // Handle stores
        if (storesResult.status === 'fulfilled') {
          console.log('[Observability] Stores loaded:', storesResult.value);
          const storesData = storesResult.value || [];
          setStores(storesData);
          if (storesData.length > 0 && !selectedStore) {
            setSelectedStore(storesData[0].name);
          }
        } else {
          console.error('[Observability] Failed to load stores:', storesResult.reason);
        }

        // Check if all failed
        if (statsResult.status === 'rejected' && 
            telemetryResult.status === 'rejected' && 
            storesResult.status === 'rejected') {
          const errMsg = statsResult.reason instanceof Error 
            ? statsResult.reason.message 
            : 'Failed to connect to API';
          setError(errMsg + ' - Is the API server running?');
        }
      } catch (err) {
        console.error('[Observability] Unexpected error:', err);
        setError(err instanceof Error ? err.message : 'Failed to load observability data');
      } finally {
        setLoading(false);
      }
    };
    fetchData();

    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [selectedStore]);

  // Fetch store-specific data when store changes
  useEffect(() => {
    if (!selectedStore) return;

    const fetchStoreData = async () => {
      setQueriesLoading(true);
      try {
        const [statsData, queriesData] = await Promise.all([
          getQueryStats(selectedStore, 7),
          getRecentQueries(selectedStore, 50),
        ]);
        setQueryStats(statsData);
        setRecentQueries(queriesData.queries);
      } catch (err) {
        console.error('Failed to load store data:', err);
      } finally {
        setQueriesLoading(false);
      }
    };
    fetchStoreData();
  }, [selectedStore]);

  if (loading) {
    return (
      <main className="main admin-page">
        <LoadingSpinner size="large" message="Loading observability data..." />
      </main>
    );
  }

  return (
    <main className="main admin-page">
      {/* Header */}
      <div className="admin-header">
        <Link href="/admin" className="back-link">
          ← Back to Admin
        </Link>
        <h1>📊 Observability Dashboard</h1>
        <p className="admin-subtitle">Real-time telemetry and search performance metrics</p>
      </div>

      {/* Connection Warning */}
      <ConnectionWarning />

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {/* Global Stats Cards */}
      <div className="stats-cards stats-cards-6">
        <StatCard
          icon="🔍"
          label="Total Queries"
          value={formatNumber(stats?.telemetry.totalQueries ?? 0, 0)}
        />
        <StatCard
          icon="⚡"
          label="Avg Latency"
          value={formatMs(stats?.telemetry.avgLatencyMs ?? 0)}
          subtitle={`P95: ${formatMs(stats?.telemetry.p95LatencyMs ?? 0)}`}
        />
        <StatCard
          icon="💾"
          label="Cache Hit Rate"
          value={formatPercent(stats?.telemetry.cacheHitRate ?? 0)}
        />
        <StatCard
          icon="🔄"
          label="Rerank Skip Rate"
          value={formatPercent(stats?.telemetry.rerankSkipRate ?? 0)}
        />
        <StatCard
          icon="📈"
          label="P50 Latency"
          value={formatMs(stats?.telemetry.p50LatencyMs ?? 0)}
        />
        <StatCard
          icon="🚀"
          label="P99 Latency"
          value={formatMs(stats?.telemetry.p99LatencyMs ?? 0)}
        />
      </div>

      {/* Distribution Charts */}
      <div className="charts-row">
        <div className="admin-card">
          <DistributionChart
            title="Query Intent Distribution"
            data={stats?.intents ?? {}}
            colorScheme="blue"
          />
        </div>
        <div className="admin-card">
          <DistributionChart
            title="Strategy Distribution"
            data={stats?.strategies ?? {}}
            colorScheme="green"
          />
        </div>
      </div>

      {/* Store Selector */}
      <div className="admin-card">
        <div className="store-selector">
          <label className="filter-label">Select Store for Details</label>
          <select
            className="filter-input"
            value={selectedStore}
            onChange={(e) => setSelectedStore(e.target.value)}
          >
            {stores.map((store) => (
              <option key={store.name} value={store.name}>
                {store.name}
              </option>
            ))}
          </select>
        </div>

        {queryStats && (
          <div className="store-stats-row">
            <div className="store-stat">
              <span className="store-stat-label">Total Queries (7d)</span>
              <span className="store-stat-value">{queryStats.totalQueries}</span>
            </div>
            <div className="store-stat">
              <span className="store-stat-label">Unique Queries</span>
              <span className="store-stat-value">{queryStats.uniqueQueries}</span>
            </div>
            <div className="store-stat">
              <span className="store-stat-label">Avg Latency</span>
              <span className="store-stat-value">{formatMs(queryStats.avgLatencyMs)}</span>
            </div>
            <div className="store-stat">
              <span className="store-stat-label">Avg Results</span>
              <span className="store-stat-value">{queryStats.avgResultCount.toFixed(1)}</span>
            </div>
          </div>
        )}
      </div>

      {/* Recent Queries */}
      <div className="admin-card">
        <h2 className="admin-card-title">🔎 Recent Queries ({selectedStore})</h2>
        <RecentQueriesTable queries={recentQueries} loading={queriesLoading} />
      </div>

      {/* Telemetry Records */}
      <div className="admin-card">
        <h2 className="admin-card-title">📡 Telemetry Records (All Stores)</h2>
        <TelemetryTable records={telemetry} loading={false} />
      </div>
    </main>
  );
}
