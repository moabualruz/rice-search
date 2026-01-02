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

  const [showSettings, setShowSettings] = useState(false);

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
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-bold text-white">Mission Control</h1>
        <button
          onClick={() => setShowSettings(true)}
          className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors border border-slate-600"
        >
          <span>⚙️</span>
          <span>System Settings</span>
        </button>
      </div>

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

      {/* Quick Actions & Info */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
           <h2 className="text-xl font-semibold text-white mb-4">Quick Actions</h2>
           <div className="grid grid-cols-2 gap-4">
             <Link href="/admin/models" className="col-span-2 block px-4 py-3 bg-purple-900/50 text-purple-200 border border-purple-700/50 rounded-lg hover:bg-purple-900/80 transition-colors text-center font-semibold">
               Manage Model Registry
             </Link>
             <button 
               onClick={rebuildIndex}
               className="px-4 py-3 bg-primary text-white rounded-lg hover:bg-accent transition-colors font-medium"
             >
               Rebuild Index
             </button>
             <button 
               onClick={clearCache}
               className="px-4 py-3 bg-slate-700 text-white rounded-lg hover:bg-slate-600 transition-colors font-medium"
             >
               Clear Cache
             </button>
           </div>
        </div>

        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <div className="flex justify-between items-center mb-4">
             <h2 className="text-xl font-semibold text-white">Feature Management</h2>
             <button onClick={() => setShowSettings(true)} className="text-xs text-primary hover:text-white transition-colors uppercase font-bold tracking-wider">
               Configure
             </button>
          </div>
          <div className="space-y-3">
             <FeatureStatus label="Hybrid Search" enabled={adminStatus?.features?.hybrid_search} />
             <FeatureStatus label="Neural Reranker" enabled={adminStatus?.features?.rerank_enabled ?? true} />
             <FeatureStatus label="AST Parsing" enabled={adminStatus?.features?.ast_parsing} />
             <FeatureStatus label="MCP Protocol" enabled={adminStatus?.features?.mcp_protocol} />
          </div>
        </div>
      </div>

      {showSettings && (
        <SettingsModal onClose={() => { setShowSettings(false); fetchData(); }} />
      )}
    </div>
  );
}

function FeatureStatus({ label, enabled }: { label: string; enabled?: boolean }) {
  return (
    <div className="flex items-center justify-between p-3 bg-slate-900/50 rounded-lg border border-slate-700/50">
      <span className="text-slate-300">{label}</span>
      <div className={`flex items-center gap-2 px-2 py-1 rounded text-xs font-medium ${
        enabled ? 'bg-green-500/10 text-green-400' : 'bg-slate-700/50 text-slate-500'
      }`}>
        <div className={`w-1.5 h-1.5 rounded-full ${enabled ? 'bg-green-400' : 'bg-slate-500'}`} />
        {enabled ? 'Active' : 'Disabled'}
      </div>
    </div>
  );
}

function StatusCard({ title, status, statusColor, icon, detail }: any) {
    const colorClasses: any = {
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

// Reuse Config Page Logic in Modal
function SettingsModal({ onClose }: { onClose: () => void }) {
  const [config, setConfig] = useState<Record<string, any>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetch(`${API_BASE}/config`)
      .then(res => res.json())
      .then(data => {
        setConfig(data);
        setLoading(false);
      });
  }, []);

  const saveConfig = async () => {
    setSaving(true);
    try {
      await fetch(`${API_BASE}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
           sparse_enabled: config.sparse_enabled,
           rrf_k: config.rrf_k,
           ast_parsing_enabled: config.ast_parsing_enabled,
           mcp_enabled: config.mcp_enabled,
           worker_pool: config.worker_pool,
           worker_concurrency: config.worker_concurrency,
           model_auto_unload: config.model_auto_unload,
           model_ttl_seconds: config.model_ttl_seconds,
           mcp_transport: config.mcp_transport,
           mcp_tcp_port: config.mcp_tcp_port
        })
      });
      onClose();
    } catch (e) {
      console.error(e);
      setSaving(false);
    }
  };

  const updateConfig = (k: string, v: any) => setConfig(prev => ({...prev, [k]: v}));

  if (loading) return null; // Or spinner inside modal

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-slate-800 rounded-xl border border-slate-700 w-full max-w-2xl shadow-2xl overflow-hidden max-h-[90vh] flex flex-col">
        <div className="p-6 border-b border-slate-700 flex justify-between items-center bg-slate-900/50">
          <h2 className="text-xl font-bold text-white">System Settings</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white">✕</button>
        </div>
        
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          
          {/* Features Section */}
          <section>
            <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">Core Features</h3>
            <div className="space-y-3">
              <ToggleSetting 
                label="Hybrid Search (SPLADE)" 
                desc="Enables sparse vector generation for better keyword matching."
                enabled={config.sparse_enabled ?? true} 
                onToggle={(v) => updateConfig('sparse_enabled', v)} 
              />
              <ToggleSetting 
                label="AST Parsing" 
                desc="Uses Tree-sitter to parse code structure for better chunking."
                enabled={config.ast_parsing_enabled ?? true} 
                onToggle={(v) => updateConfig('ast_parsing_enabled', v)} 
              />
               <ToggleSetting 
                label="MCP Protocol" 
                desc="Enables Model Context Protocol server."
                enabled={config.mcp_enabled ?? false} 
                onToggle={(v) => updateConfig('mcp_enabled', v)} 
              />
              {config.mcp_enabled && (
                  <div className="mt-3 ml-4 pl-4 border-l border-slate-700 grid grid-cols-2 gap-4">
                      <div>
                          <label className="block text-xs font-medium text-slate-400 mb-1">Transport</label>
                          <select 
                            value={config.mcp_transport ?? 'stdio'}
                            onChange={(e) => updateConfig('mcp_transport', e.target.value)}
                            className="w-full bg-slate-900 border border-slate-700 rounded px-2 py-1 text-white text-sm"
                          >
                             <option value="stdio">STDIO (Local)</option>
                             <option value="tcp">TCP (Network)</option>
                             <option value="sse">SSE (Web)</option>
                          </select>
                      </div>
                      {config.mcp_transport === 'tcp' && (
                          <div>
                              <label className="block text-xs font-medium text-slate-400 mb-1">Port</label>
                              <input 
                                type="number" 
                                value={config.mcp_tcp_port ?? 9090}
                                onChange={(e) => updateConfig('mcp_tcp_port', parseInt(e.target.value))}
                                className="w-full bg-slate-900 border border-slate-700 rounded px-2 py-1 text-white text-sm"
                              />
                          </div>
                      )}
                  </div>
              )}
            </div>
          </section>

          {/* Tuning Section */}
          <section>
            <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">Performance & Resources</h3>
            <div className="space-y-4">
               
               {/* RRF Tuning */}
               <div>
                  <div className="flex items-center gap-2 mb-1">
                      <label className="block text-sm font-medium text-slate-300">RRF Constant (k)</label>
                      <div className="group relative cursor-help">
                          <span className="text-slate-500 text-xs border border-slate-600 rounded-full w-4 h-4 flex items-center justify-center">?</span>
                          <div className="hidden group-hover:block absolute left-full top-0 ml-2 w-64 p-2 bg-black border border-slate-700 rounded text-xs text-slate-300 z-50">
                             Controls the rank fusion balance. Higher values (e.g. 60) favor stability, lower values favor high-ranking hits.
                          </div>
                      </div>
                  </div>
                  <input 
                    type="number" 
                    value={config.rrf_k ?? 60}
                    onChange={(e) => updateConfig('rrf_k', parseInt(e.target.value))}
                    className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                  />
               </div>

               {/* Model Memory Management */}
               <div className="p-4 bg-slate-900/40 rounded-lg border border-slate-800">
                  <h4 className="text-sm font-medium text-slate-200 mb-3">Memory Optimization</h4>
                  <ToggleSetting 
                    label="Auto-Unload Models" 
                    desc="Free up GPU/RAM by unloading unused models."
                    enabled={config.model_auto_unload ?? true} 
                    onToggle={(v) => updateConfig('model_auto_unload', v)} 
                  />
                  {config.model_auto_unload && (
                      <div className="mt-3 ml-1">
                        <label className="block text-xs font-medium text-slate-400 mb-1">Idle Timeout (Seconds)</label>
                        <div className="flex gap-2">
                            <input 
                                type="number" 
                                value={config.model_ttl_seconds ?? 300}
                                onChange={(e) => updateConfig('model_ttl_seconds', parseInt(e.target.value))}
                                className="w-24 bg-slate-900 border border-slate-700 rounded px-2 py-1 text-white text-sm"
                            />
                            <span className="text-xs text-slate-500 self-center">default: 300s (5m)</span>
                        </div>
                      </div>
                  )}
               </div>
               
               <div className="grid grid-cols-2 gap-4">
                 <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Worker Pool</label>
                    <select 
                      value={config.worker_pool ?? 'solo'}
                      onChange={(e) => updateConfig('worker_pool', e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                    >
                      <option value="solo">Solo (Debug)</option>
                      <option value="threads">Threads</option>
                      <option value="gevent">Gevent (Async)</option>
                    </select>
                 </div>
                 <div>
                    <label className="block text-sm font-medium text-slate-300 mb-1">Concurrency</label>
                    <input 
                      type="number" 
                      value={config.worker_concurrency ?? 1}
                      onChange={(e) => updateConfig('worker_concurrency', parseInt(e.target.value))}
                      className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                    />
                 </div>
               </div>
            </div>
          </section>

        </div>

        <div className="p-6 border-t border-slate-700 bg-slate-900/50 flex justify-end gap-3">
          <button 
            onClick={onClose}
            className="px-4 py-2 text-slate-300 hover:text-white transition-colors"
          >
            Cancel
          </button>
          <button 
            onClick={saveConfig}
            disabled={saving}
            className="px-6 py-2 bg-primary text-white rounded-lg hover:bg-accent transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  );
}

function ToggleSetting({ label, desc, enabled, onToggle }: any) {
  return (
    <div className="flex items-center justify-between p-3 rounded-lg hover:bg-slate-700/30 transition-colors">
      <div>
        <div className="font-medium text-slate-200">{label}</div>
        <div className="text-xs text-slate-500">{desc}</div>
      </div>
      <button 
        onClick={() => onToggle(!enabled)}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-slate-900 ${
          enabled ? 'bg-primary' : 'bg-slate-600'
        }`}
      >
        <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${enabled ? 'translate-x-6' : 'translate-x-1'}`} />
      </button>
    </div>
  );
}
