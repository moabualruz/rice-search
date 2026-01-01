'use client';

import { useState, useEffect } from 'react';
import Image from 'next/image';
import Link from 'next/link';

interface SystemStatus {
  status: string;
  components: {
    qdrant?: { status: string; collections?: number };
    celery?: { status: string };
  };
}

interface AdminStatus {
  status: string;
  features: Record<string, any>;
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

  const toggleFeature = async (key: string, value: boolean) => {
    try {
       // Optimistic update
       setAdminStatus(prev => prev ? {
         ...prev,
         features: {
           ...prev.features,
           [key === 'sparse_enabled' ? 'hybrid_search' : 
            key === 'ast_parsing_enabled' ? 'ast_parsing' : 
            key === 'mcp_enabled' ? 'mcp_protocol' : key]: value
         }
       } : null);

       const res = await fetch(`${API_BASE}/config`, {
         method: 'PUT',
         headers: { 'Content-Type': 'application/json' },
         body: JSON.stringify({ [key]: value })
       });
       
       if (res.ok) {
         const data = await res.json();
         showMessage('success', `${key} updated. ${data.restart_required ? 'Restart required.' : ''}`);
         // Refetch to ensure consistency
         setTimeout(fetchData, 500);
       } else {
         throw new Error('Update failed');
       }
    } catch (e) {
      showMessage('error', `Failed to update ${key}`);
      fetchData(); // Revert
    }
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

      {/* Configuration Management */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h2 className="text-xl font-semibold text-white mb-4">Feature Management</h2>
          <div className="space-y-4">
            <ToggleRow 
              label="Hybrid Search (SPLADE)" 
              enabled={adminStatus?.features?.hybrid_search ?? false} 
              onToggle={() => toggleFeature('sparse_enabled', !adminStatus?.features?.hybrid_search)}
            />
            <ToggleRow 
              label="Neural Reranker" 
              enabled={adminStatus?.features?.rerank_enabled ?? true} 
              onToggle={() => toggleFeature('rerank_enabled', !adminStatus?.features?.rerank_enabled)}
            />
            <div className="pl-8 text-xs text-slate-500 mb-2">
               Model: {adminStatus?.features?.rerank_model || 'jinaai/jina-reranker-v2-base-multilingual'}
            </div>
            
            <ToggleRow 
              label="AST Parsing (Tree-sitter)" 
              enabled={adminStatus?.features?.ast_parsing ?? false} 
              onToggle={() => toggleFeature('ast_parsing_enabled', !adminStatus?.features?.ast_parsing)}
            />
            <ToggleRow 
              label="MCP Protocol" 
              enabled={adminStatus?.features?.mcp_protocol ?? false} 
              onToggle={() => toggleFeature('mcp_enabled', !adminStatus?.features?.mcp_protocol)}
            />
          </div>
        </div>

        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h2 className="text-xl font-semibold text-white mb-4">Quick Actions</h2>
          <div className="space-y-3">
             <Link href="/admin/models" className="w-full block px-4 py-2 bg-purple-900/50 text-purple-200 border border-purple-700/50 rounded-lg hover:bg-purple-900/80 transition-colors text-center font-semibold">
               Manage Model Registry
             </Link>
             <div className="h-px bg-slate-700 my-2" />
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

function ToggleRow({ label, enabled, onToggle }: { label: string; enabled: boolean; onToggle: () => void }) {
  return (
    <div className="flex items-center justify-between p-2 rounded hover:bg-slate-700/50 transition-colors">
      <span className="text-slate-300 font-medium">{label}</span>
      <button 
        onClick={onToggle}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-slate-900 ${
          enabled ? 'bg-primary' : 'bg-slate-600'
        }`}
      >
        <span
          className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
            enabled ? 'translate-x-6' : 'translate-x-1'
          }`}
        />
      </button>
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
