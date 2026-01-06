'use client';

import { useState, useEffect } from 'react';
import Image from 'next/image';
import Link from 'next/link';

interface AdminStatus {
  status: string;
  features: Record<string, any>;
  models: Record<string, boolean>;
  components: {
    qdrant?: { status: string; collections?: number };
    celery?: { status: string };
    redis?: { status: string };
    minio?: { status: string };
  };
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

export default function AdminDashboard() {
  const [adminStatus, setAdminStatus] = useState<AdminStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const [showSettings, setShowSettings] = useState(false);

  const fetchData = async () => {
    try {
      const statusRes = await fetch(`${API_BASE}/system/status`);
      if (statusRes.ok) {
         setAdminStatus(await statusRes.json());
      }
    } catch (e) {
      console.error(e);
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
        <div className="flex items-center gap-3">
          <Link
            href="/admin/settings"
            className="flex items-center gap-2 px-4 py-2 bg-primary hover:bg-accent text-white rounded-lg transition-colors border border-primary/30"
          >
            <span>⚙️</span>
            <span>Settings Manager</span>
          </Link>
          <button
            onClick={() => setShowSettings(true)}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors border border-slate-600"
          >
            <span>🎛️</span>
            <span>Quick Config</span>
          </button>
        </div>
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

      <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-8">
        <StatusCard
          title="System"
          status={adminStatus?.status === 'ok' || adminStatus?.status === 'healthy' ? 'Healthy' : 'Degraded'}
          statusColor={adminStatus?.status === 'ok' || adminStatus?.status === 'healthy' ? 'green' : 'yellow'}
          icon="🖥️"
        />
        <StatusCard
          title="Qdrant"
          status={adminStatus?.components?.qdrant?.status || 'Unknown'}
          statusColor={adminStatus?.components?.qdrant?.status === 'up' ? 'green' : 'red'}
          icon="🗄️"
          detail={adminStatus?.components?.qdrant?.collections ? `${adminStatus.components.qdrant.collections} collections` : undefined}
        />
        <StatusCard
          title="Workers"
          status={adminStatus?.components?.celery?.status || 'Unknown'}
          statusColor={adminStatus?.components?.celery?.status === 'up' ? 'green' : 'red'}
          icon="⚡"
        />
        <StatusCard
          title="Redis"
          status={adminStatus?.components?.redis?.status || 'Unknown'}
          statusColor={adminStatus?.components?.redis?.status === 'up' ? 'green' : 'red'}
          icon="🔄"
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
             <Link href="/admin/settings" className="text-xs text-primary hover:text-white transition-colors uppercase font-bold tracking-wider">
               Configure →
             </Link>
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

// Quick Config Modal - Most Important Settings
function SettingsModal({ onClose }: { onClose: () => void }) {
  const [settings, setSettings] = useState<Record<string, any>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const SETTINGS_API = 'http://localhost:8000/api/v1/settings';

  useEffect(() => {
    // Fetch specific important settings
    const fetchSettings = async () => {
      try {
        const keys = [
          'search.hybrid.use_bm25',
          'search.hybrid.use_splade',
          'search.hybrid.use_bm42',
          'models.reranker.enabled',
          'ast.enabled',
          'search.hybrid.rrf_k',
          'search.default_limit',
          'indexing.chunk_size',
          'model_management.auto_unload',
          'model_management.ttl_seconds',
        ];

        const results: Record<string, any> = {};
        await Promise.all(
          keys.map(async (key) => {
            const res = await fetch(`${SETTINGS_API}/${key}`);
            if (res.ok) {
              const data = await res.json();
              results[key] = data.value;
            }
          })
        );

        setSettings(results);
        setLoading(false);
      } catch (e) {
        console.error('Failed to load settings:', e);
        setLoading(false);
      }
    };

    fetchSettings();
  }, []);

  const updateSetting = async (key: string, value: any) => {
    try {
      const res = await fetch(`${SETTINGS_API}/${key}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value })
      });

      if (res.ok) {
        setSettings(prev => ({ ...prev, [key]: value }));
      }
    } catch (e) {
      console.error(`Failed to update ${key}:`, e);
    }
  };

  const saveAll = async () => {
    setSaving(true);
    try {
      // All individual updates have already been persisted
      // Just close the modal
      onClose();
    } catch (e) {
      console.error(e);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <p className="text-slate-300">Loading settings...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="bg-slate-800 rounded-xl border border-slate-700 w-full max-w-2xl shadow-2xl overflow-hidden max-h-[90vh] flex flex-col">
        <div className="p-6 border-b border-slate-700 flex justify-between items-center bg-slate-900/50">
          <h2 className="text-xl font-bold text-white">System Settings</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-white">✕</button>
        </div>
        
        <div className="flex-1 overflow-y-auto p-6 space-y-6">

          {/* Retrieval Methods */}
          <section>
            <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">🔍 Triple Retrieval System</h3>
            <div className="space-y-3">
              <ToggleSetting
                label="BM25 Lexical Search"
                desc="Traditional keyword-based search via Tantivy (Rust)."
                enabled={settings['search.hybrid.use_bm25'] ?? true}
                onToggle={(v) => updateSetting('search.hybrid.use_bm25', v)}
              />
              <ToggleSetting
                label="SPLADE Neural Sparse"
                desc="Learned sparse vectors for improved keyword matching."
                enabled={settings['search.hybrid.use_splade'] ?? true}
                onToggle={(v) => updateSetting('search.hybrid.use_splade', v)}
              />
              <ToggleSetting
                label="BM42 Hybrid Search"
                desc="Qdrant's native sparse+dense hybrid retrieval."
                enabled={settings['search.hybrid.use_bm42'] ?? true}
                onToggle={(v) => updateSetting('search.hybrid.use_bm42', v)}
              />
            </div>
          </section>

          {/* Search Enhancements */}
          <section>
            <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">⚡ Search Enhancements</h3>
            <div className="space-y-3">
              <ToggleSetting
                label="Neural Reranking"
                desc="Use cross-encoder to rerank results for better relevance."
                enabled={settings['models.reranker.enabled'] ?? true}
                onToggle={(v) => updateSetting('models.reranker.enabled', v)}
              />
              <ToggleSetting
                label="AST-Aware Parsing"
                desc="Parse code structure for better chunking (Tree-sitter)."
                enabled={settings['ast.enabled'] ?? true}
                onToggle={(v) => updateSetting('ast.enabled', v)}
              />
            </div>
          </section>

          {/* Tuning Parameters */}
          <section>
            <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">🎛️ Performance Tuning</h3>
            <div className="space-y-4">

               {/* RRF Tuning */}
               <div>
                  <div className="flex items-center gap-2 mb-1">
                      <label className="block text-sm font-medium text-slate-300">RRF Constant (k)</label>
                      <div className="group relative cursor-help">
                          <span className="text-slate-500 text-xs border border-slate-600 rounded-full w-4 h-4 flex items-center justify-center">?</span>
                          <div className="hidden group-hover:block absolute left-full top-0 ml-2 w-64 p-2 bg-black border border-slate-700 rounded text-xs text-slate-300 z-50">
                             Controls rank fusion balance. Higher (60+) favors stability, lower favors top matches.
                          </div>
                      </div>
                  </div>
                  <input
                    type="number"
                    value={settings['search.hybrid.rrf_k'] ?? 60}
                    onChange={(e) => updateSetting('search.hybrid.rrf_k', parseInt(e.target.value))}
                    onBlur={(e) => updateSetting('search.hybrid.rrf_k', parseInt(e.target.value))}
                    className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                  />
               </div>

               {/* Search Limit */}
               <div>
                  <label className="block text-sm font-medium text-slate-300 mb-1">Default Result Limit</label>
                  <input
                    type="number"
                    value={settings['search.default_limit'] ?? 10}
                    onChange={(e) => updateSetting('search.default_limit', parseInt(e.target.value))}
                    onBlur={(e) => updateSetting('search.default_limit', parseInt(e.target.value))}
                    className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                  />
               </div>

               {/* Chunk Size */}
               <div>
                  <label className="block text-sm font-medium text-slate-300 mb-1">Chunk Size (characters)</label>
                  <input
                    type="number"
                    value={settings['indexing.chunk_size'] ?? 1000}
                    onChange={(e) => updateSetting('indexing.chunk_size', parseInt(e.target.value))}
                    onBlur={(e) => updateSetting('indexing.chunk_size', parseInt(e.target.value))}
                    className="w-full bg-slate-900 border border-slate-700 rounded px-3 py-2 text-white"
                  />
               </div>

               {/* Model Memory Management */}
               <div className="p-4 bg-slate-900/40 rounded-lg border border-slate-800">
                  <h4 className="text-sm font-medium text-slate-200 mb-3">Memory Optimization</h4>
                  <ToggleSetting
                    label="Auto-Unload Models"
                    desc="Unload idle models to free GPU/RAM."
                    enabled={settings['model_management.auto_unload'] ?? true}
                    onToggle={(v) => updateSetting('model_management.auto_unload', v)}
                  />
                  {settings['model_management.auto_unload'] && (
                      <div className="mt-3 ml-1">
                        <label className="block text-xs font-medium text-slate-400 mb-1">Idle Timeout (Seconds)</label>
                        <div className="flex gap-2">
                            <input
                                type="number"
                                value={settings['model_management.ttl_seconds'] ?? 300}
                                onChange={(e) => updateSetting('model_management.ttl_seconds', parseInt(e.target.value))}
                                onBlur={(e) => updateSetting('model_management.ttl_seconds', parseInt(e.target.value))}
                                className="w-24 bg-slate-900 border border-slate-700 rounded px-2 py-1 text-white text-sm"
                            />
                            <span className="text-xs text-slate-500 self-center">default: 300s (5m)</span>
                        </div>
                      </div>
                  )}
               </div>
            </div>
          </section>

          <div className="text-xs text-slate-500 text-center py-2 border-t border-slate-700">
            💡 Changes are automatically saved to settings.yaml
          </div>

        </div>

        <div className="p-6 border-t border-slate-700 bg-slate-900/50 flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-6 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
          >
            Close
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
