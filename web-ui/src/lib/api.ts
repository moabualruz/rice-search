import type {
  StoreInfo,
  StoreStats,
  IndexStats,
  SearchRequest,
  SearchResponse,
  ListFilesResponse,
  SortField,
  SortOrder,
  ObservabilityStats,
  QueryStats,
  RecentQueriesResponse,
  TelemetryResponse,
} from '@/types';

/**
 * API client for Rice Search
 * All requests go through /api proxy (configured in next.config.js)
 */

const API_BASE = '/api';

class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    throw new ApiError(`Request failed: ${response.statusText}`, response.status);
  }

  return response.json();
}

// ============================================================================
// Stores API
// ============================================================================

export async function listStores(): Promise<StoreInfo[]> {
  const data = await request<{ stores: StoreInfo[] }>('/v1/stores');
  return data.stores || [];
}

export async function createStore(name: string, description?: string): Promise<StoreInfo> {
  return request<StoreInfo>('/v1/stores', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  });
}

export async function getStore(store: string): Promise<StoreInfo> {
  return request<StoreInfo>(`/v1/stores/${store}`);
}

export async function deleteStore(store: string): Promise<void> {
  await request(`/v1/stores/${store}`, { method: 'DELETE' });
}

export async function getStoreStats(store: string): Promise<StoreStats> {
  return request<StoreStats>(`/v1/stores/${store}/stats`);
}

// ============================================================================
// Index API
// ============================================================================

export async function getIndexStats(store: string): Promise<IndexStats> {
  return request<IndexStats>(`/v1/stores/${store}/index/stats`);
}

export async function listFiles(
  store: string,
  options: {
    page?: number;
    pageSize?: number;
    pathFilter?: string;
    language?: string;
    sortBy?: SortField;
    sortOrder?: SortOrder;
  } = {},
): Promise<ListFilesResponse> {
  const params = new URLSearchParams();
  if (options.page) params.set('page', String(options.page));
  if (options.pageSize) params.set('page_size', String(options.pageSize));
  if (options.pathFilter) params.set('path', options.pathFilter);
  if (options.language) params.set('language', options.language);
  if (options.sortBy) params.set('sort_by', options.sortBy);
  if (options.sortOrder) params.set('sort_order', options.sortOrder);

  const query = params.toString();
  return request<ListFilesResponse>(`/v1/stores/${store}/index/files${query ? `?${query}` : ''}`);
}

// ============================================================================
// Search API
// ============================================================================

export async function search(store: string, searchRequest: SearchRequest): Promise<SearchResponse> {
  return request<SearchResponse>(`/v1/stores/${store}/search`, {
    method: 'POST',
    body: JSON.stringify(searchRequest),
  });
}

// ============================================================================
// Observability API
// ============================================================================

export async function getObservabilityStats(): Promise<ObservabilityStats> {
  return request<ObservabilityStats>('/v1/observability/stats');
}

export async function getQueryStats(store: string, days = 7): Promise<QueryStats> {
  return request<QueryStats>(`/v1/observability/query-stats?store=${store}&days=${days}`);
}

export async function getRecentQueries(store: string, limit = 50): Promise<RecentQueriesResponse> {
  return request<RecentQueriesResponse>(`/v1/observability/recent-queries?store=${store}&limit=${limit}`);
}

export async function getTelemetryRecords(limit = 100, store?: string): Promise<TelemetryResponse> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (store) params.set('store', store);
  return request<TelemetryResponse>(`/v1/observability/telemetry?${params.toString()}`);
}

// ============================================================================
// Health API
// ============================================================================

export interface HealthStatus {
  status: string;
  uptime?: number;
  version?: string;
}

export async function getHealth(): Promise<HealthStatus> {
  return request<HealthStatus>('/healthz');
}

export async function getDetailedHealth(): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>('/v1/health');
}

// Re-export types for convenience
export type { ApiError };
