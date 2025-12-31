'use client';

import { useState, useEffect } from 'react';
import Image from 'next/image';

interface SystemStatus {
  status: string;
  components: {
    qdrant?: { status: string; collections?: number };
    celery?: { status: string };
  };
}

interface AdminStatus {
  status: string;
  features: Record<string, boolean>;
  models: Record<string, boolean>;
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

export default function AdminDashboard() {
  const [health, setHealth] = useState<SystemStatus | null>(null);
  const [adminStatus, setAdminStatus] = useState<AdminStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const fetchData = async () => {
    try {
      const [healthRes, statusRes] = await Promise.all([
        fetch('http://localhost:8000/health'),
        fetch(`${API_BASE}/system/status`)
      ]);
      
      if (healthRes.ok) setHealth(await healthRes.json());
      if (statusRes.ok) setAdminStatus(await statusRes.json());
    } catch (e) {
      setHealth({ status: 'error', components: {} });
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchData();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const rebuildIndex = async () => {
    try {
      const res = await fetch(`${API_BASE}/system/rebuild-index`, { method: 'POST' });
      if (res.ok) showMessage('success', 'Index rebuild triggered');
    } catch (e) {
      showMessage('error', 'Failed to trigger rebuild');
    }
  };

  const clearCache = async () => {
    try {
      const res = await fetch(`${API_BASE}/system/clear-cache`, { method: 'POST' });
      if (res.ok) showMessage('success', 'Cache clear triggered');
    } catch (e) {
      showMessage('error', 'Failed to clear cache');
    }
  };

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-8">Mission Control</h1>

      {message && (
        <div className={`mb-6 p-4 rounded-lg ${
          message.type === 'success' 
            ? 'bg-green-600/20 border border-green-500/30 text-green-400'
            : 'bg-red-600/20 border border-red-500/30 text-red-400'
        }`}>
          {message.type === 'success' ? '✓' : '✗'} {message.text}
        </div>
      )}

      {/* System Status */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <StatusCard
          title="System Status"
          status={health?.status === 'ok' ? 'Healthy' : 'Degraded'}
          statusColor={health?.status === 'ok' ? 'green' : 'yellow'}
          icon="🖥️"
        />
        <StatusCard
          title="Qdrant"
          status={health?.components?.qdrant?.status || 'Unknown'}
          statusColor={health?.components?.qdrant?.status === 'up' ? 'green' : 'red'}
          icon="🗄️"
          detail={`${health?.components?.qdrant?.collections || 0} collections`}
        />
        <StatusCard
          title="Workers"
          status={health?.components?.celery?.status || 'Unknown'}
          statusColor={health?.components?.celery?.status === 'up' ? 'green' : 'red'}
          icon="⚡"
        />
      </div>

      {/* Quick Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h2 className="text-xl font-semibold text-white mb-4">Active Features</h2>
          <div className="space-y-3">
            <FeatureRow label="Hybrid Search (SPLADE)" enabled={adminStatus?.features?.hybrid_search ?? true} />
            <FeatureRow label="AST Parsing" enabled={adminStatus?.features?.ast_parsing ?? true} />
            <FeatureRow label="MCP Protocol" enabled={adminStatus?.features?.mcp_protocol ?? false} />
            <FeatureRow label="OpenTelemetry" enabled={adminStatus?.features?.opentelemetry ?? false} />
          </div>
        </div>

        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h2 className="text-xl font-semibold text-white mb-4">Quick Actions</h2>
          <div className="space-y-3">
            <button 
              onClick={rebuildIndex}
              className="w-full px-4 py-2 bg-primary text-white rounded-lg hover:bg-accent transition-colors"
            >
              Rebuild Index
            </button>
            <button 
              onClick={clearCache}
              className="w-full px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 transition-colors"
            >
              Clear Cache
            </button>
            <button 
              onClick={fetchData}
              className="w-full px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 transition-colors"
            >
              Refresh Status
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatusCard({
  title,
  status,
  statusColor,
  icon,
  detail,
}: {
  title: string;
  status: string;
  statusColor: 'green' | 'yellow' | 'red';
  icon: string;
  detail?: string;
}) {
  const colorClasses = {
    green: 'bg-green-500/20 text-green-400 border-green-500/30',
    yellow: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
    red: 'bg-red-500/20 text-red-400 border-red-500/30',
  };

  return (
    <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
      <div className="flex items-center justify-between mb-4">
        <span className="text-2xl">{icon}</span>
        <span className={`px-3 py-1 rounded-full text-sm border ${colorClasses[statusColor]}`}>
          {status}
        </span>
      </div>
      <h3 className="text-lg font-medium text-white">{title}</h3>
      {detail && <p className="text-slate-400 text-sm mt-1">{detail}</p>}
    </div>
  );
}

function FeatureRow({ label, enabled }: { label: string; enabled: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-slate-300">{label}</span>
      <span className={enabled ? 'text-green-400' : 'text-slate-500'}>
        {enabled ? '✓ Enabled' : '○ Disabled'}
      </span>
    </div>
  );
}
