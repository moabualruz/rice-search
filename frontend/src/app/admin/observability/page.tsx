'use client';

import { useState, useEffect } from 'react';

interface AuditLog {
  id: string;
  timestamp: string;
  action: string;
  user: string;
}

interface Metrics {
  search_latency_p50_ms: number;
  search_latency_p95_ms: number;
  search_latency_p99_ms: number;
  index_rate_docs_per_sec: number;
  active_connections: number;
  gpu_memory_used_mb: number;
  gpu_memory_total_mb: number;
  gpu_utilization_percent: number;
  cpu_usage_percent: number;
  memory_usage_mb: number;
  components: Record<string, string>;
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

export default function ObservabilityPage() {
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = async () => {
    try {
      const [metricsRes, logsRes] = await Promise.all([
        fetch(`${API_BASE}/metrics`),
        fetch(`${API_BASE}/audit-log?limit=10`)
      ]);
      
      if (metricsRes.ok) setMetrics(await metricsRes.json());
      if (logsRes.ok) {
        const data = await logsRes.json();
        setLogs(data.logs || []);
      }
    } catch (e) {
      console.error('Failed to fetch observability data', e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchData();
    // Refresh every 5 seconds for quicker updates
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, []);

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white">Observability</h1>
          <p className="text-slate-400">Real-time system health and metrics</p>
        </div>
        <button 
          onClick={fetchData}
          className="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600"
        >
          Refresh Now
        </button>
      </div>

      {/* Component Health */}
      <h2 className="text-xl font-semibold text-white mb-4">Service Status</h2>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        {['redis', 'qdrant', 'minio', 'worker'].map((component) => (
            <ComponentCard 
                key={component}
                label={component.charAt(0).toUpperCase() + component.slice(1)}
                status={metrics?.components?.[component] || 'unknown'}
            />
        ))}
      </div>

      {/* Metrics Overview */}
      <h2 className="text-xl font-semibold text-white mb-4">Performance</h2>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <MetricCard 
          label="Search P95" 
          value={`${metrics?.search_latency_p95_ms ?? 0}ms`} 
          status={metrics && metrics.search_latency_p95_ms < 200 ? 'good' : 'warning'} 
        />
        <MetricCard 
          label="Index Rate" 
          value={`${metrics?.index_rate_docs_per_sec ?? 0}/s`} 
          status="good" 
        />
        <MetricCard 
          label="GPU Utilization" 
          value={`${metrics?.gpu_utilization_percent ?? 0}%`} 
          status="good" 
        />
        <MetricCard 
          label="Connections" 
          value={`${metrics?.active_connections ?? 0}`} 
          status="good" 
        />
      </div>

      {/* Detailed Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Search Latency</h3>
          <div className="space-y-4">
            <div className="flex justify-between items-center">
              <span className="text-slate-400">P50</span>
              <span className="text-white font-mono">{metrics?.search_latency_p50_ms ?? 0}ms</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-slate-400">P95</span>
              <span className="text-white font-mono">{metrics?.search_latency_p95_ms ?? 0}ms</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-slate-400">P99</span>
              <span className="text-white font-mono">{metrics?.search_latency_p99_ms ?? 0}ms</span>
            </div>
          </div>
        </div>
        
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Resources</h3>
          <div className="space-y-4">
            <div>
              <div className="flex justify-between items-center mb-1">
                <span className="text-slate-400">CPU</span>
                <span className="text-white font-mono">{metrics?.cpu_usage_percent ?? 0}%</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className="bg-primary h-2 rounded-full" 
                  style={{ width: `${metrics?.cpu_usage_percent ?? 0}%` }}
                />
              </div>
            </div>
            <div>
              <div className="flex justify-between items-center mb-1">
                <span className="text-slate-400">Memory</span>
                <span className="text-white font-mono">{((metrics?.memory_usage_mb ?? 0) / 1024).toFixed(1)} GB</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className="bg-primary h-2 rounded-full" 
                  style={{ width: `${Math.min((metrics?.memory_usage_mb ?? 0) / 80, 100)}%` }}
                />
              </div>
            </div>
            <div>
              <div className="flex justify-between items-center mb-1">
                <span className="text-slate-400">GPU Memory</span>
                <span className="text-white font-mono">
                  {((metrics?.gpu_memory_used_mb ?? 0) / 1024).toFixed(1)} / {((metrics?.gpu_memory_total_mb ?? 0) / 1024).toFixed(0)} GB
                </span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className="bg-primary h-2 rounded-full" 
                  style={{ width: `${((metrics?.gpu_memory_used_mb ?? 0) / (metrics?.gpu_memory_total_mb ?? 8000)) * 100}%` }}
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Audit Log */}
      <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Recent Activity</h3>
        <div className="space-y-3">
          {logs.map((log) => (
            <div key={log.id} className="flex items-center gap-4 text-sm">
              <span className="text-slate-500 font-mono w-20">
                {new Date(log.timestamp).toLocaleTimeString()}
              </span>
              <span className="text-white flex-1">{log.action}</span>
              <span className={`px-2 py-1 rounded text-xs ${
                log.user === 'admin' ? 'bg-primary/20 text-primary' :
                log.user === 'system' ? 'bg-slate-700 text-slate-400' :
                'bg-slate-700 text-slate-300'
              }`}>
                {log.user}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function ComponentCard({ label, status }: { label: string; status: string }) {
    const isHealthy = status === 'healthy';
    return (
        <div className="bg-slate-800 rounded-xl p-4 border border-slate-700 flex items-center justify-between">
            <span className="text-slate-300 font-medium">{label}</span>
            <span className={`flex items-center gap-2 text-sm font-semibold capitalize ${isHealthy ? 'text-green-400' : 'text-red-400'}`}>
                <span className={`w-2 h-2 rounded-full ${isHealthy ? 'bg-green-400 animate-pulse' : 'bg-red-400'}`} />
                {status}
            </span>
        </div>
    );
}

function MetricCard({ label, value, status }: { label: string; value: string; status: 'good' | 'warning' | 'error' }) {
  const colorClass = {
    good: 'text-green-400',
    warning: 'text-yellow-400',
    error: 'text-red-400',
  }[status];

  return (
    <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
      <p className="text-slate-400 text-sm">{label}</p>
      <p className={`text-2xl font-bold ${colorClass}`}>{value}</p>
    </div>
  );
}
