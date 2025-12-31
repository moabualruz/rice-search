'use client';

import { useState, useEffect } from 'react';

interface ConfigItem {
  key: string;
  value: string | boolean | number;
  type: 'string' | 'boolean' | 'number';
  description: string;
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

export default function ConfigPage() {
  const [config, setConfig] = useState<Record<string, any>>({});
  const [loading, setLoading] = useState(true);
  const [hasChanges, setHasChanges] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const fetchConfig = async () => {
    try {
      const res = await fetch(`${API_BASE}/config`);
      if (res.ok) {
        const data = await res.json();
        setConfig(data);
      }
    } catch (e) {
      console.error('Failed to fetch config', e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const updateLocalConfig = (key: string, value: any) => {
    setConfig({ ...config, [key]: value });
    setHasChanges(true);
  };

  const saveConfig = async () => {
    try {
      const res = await fetch(`${API_BASE}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sparse_enabled: config.sparse_enabled,
          rrf_k: config.rrf_k,
          ast_parsing_enabled: config.ast_parsing_enabled,
          mcp_enabled: config.mcp_enabled
        })
      });
      if (res.ok) {
        showMessage('success', 'Configuration saved! Restart required for some changes.');
        setHasChanges(false);
      }
    } catch (e) {
      showMessage('error', 'Failed to save configuration');
    }
  };

  const configItems: ConfigItem[] = [
    { key: 'sparse_enabled', value: config.sparse_enabled ?? true, type: 'boolean', description: 'Enable hybrid search with SPLADE' },
    { key: 'sparse_model', value: config.sparse_model ?? 'naver/splade-v3', type: 'string', description: 'Sparse embedding model (read-only)' },
    { key: 'embedding_model', value: config.embedding_model ?? 'all-MiniLM-L6-v2', type: 'string', description: 'Dense embedding model (read-only)' },
    { key: 'rrf_k', value: config.rrf_k ?? 60, type: 'number', description: 'RRF fusion parameter' },
    { key: 'ast_parsing_enabled', value: config.ast_parsing_enabled ?? true, type: 'boolean', description: 'Enable Tree-sitter AST parsing' },
    { key: 'mcp_enabled', value: config.mcp_enabled ?? false, type: 'boolean', description: 'Enable MCP protocol server' },
    { key: 'mcp_transport', value: config.mcp_transport ?? 'stdio', type: 'string', description: 'MCP transport type (read-only)' },
    { key: 'mcp_tcp_port', value: config.mcp_tcp_port ?? 9090, type: 'number', description: 'MCP TCP port (read-only)' },
  ];

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white">Runtime Configuration</h1>
          <p className="text-slate-400">Edit system settings</p>
        </div>
        <div className="flex gap-3">
          <button 
            className="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600"
            onClick={fetchConfig}
          >
            Reset
          </button>
          <button 
            className={`px-6 py-2 rounded-lg ${
              hasChanges 
                ? 'bg-primary text-white hover:bg-accent' 
                : 'bg-slate-700 text-slate-400 cursor-not-allowed'
            }`}
            onClick={saveConfig}
            disabled={!hasChanges}
          >
            Save Changes
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

      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-900">
            <tr>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Setting</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Value</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Description</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {configItems.map((item) => {
              const isReadOnly = item.key.includes('model') || item.key === 'mcp_transport' || item.key === 'mcp_tcp_port';
              return (
                <tr key={item.key} className="hover:bg-slate-700/30">
                  <td className="px-6 py-4">
                    <code className="text-primary bg-slate-900 px-2 py-1 rounded">{item.key}</code>
                  </td>
                  <td className="px-6 py-4">
                    {item.type === 'boolean' ? (
                      <button
                        onClick={() => !isReadOnly && updateLocalConfig(item.key, !config[item.key])}
                        disabled={isReadOnly}
                        className={`px-4 py-1 rounded-lg text-sm ${
                          config[item.key] 
                            ? 'bg-green-500/20 text-green-400' 
                            : 'bg-slate-700 text-slate-400'
                        } ${isReadOnly ? 'opacity-50 cursor-not-allowed' : ''}`}
                      >
                        {config[item.key] ? 'Enabled' : 'Disabled'}
                      </button>
                    ) : item.type === 'number' ? (
                      <input
                        type="number"
                        value={config[item.key] ?? item.value}
                        onChange={(e) => updateLocalConfig(item.key, parseInt(e.target.value))}
                        disabled={isReadOnly}
                        className={`px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white w-24 ${isReadOnly ? 'opacity-50' : ''}`}
                      />
                    ) : (
                      <input
                        type="text"
                        value={config[item.key] ?? item.value}
                        onChange={(e) => updateLocalConfig(item.key, e.target.value)}
                        disabled={isReadOnly}
                        className={`px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white w-64 ${isReadOnly ? 'opacity-50' : ''}`}
                      />
                    )}
                  </td>
                  <td className="px-6 py-4 text-slate-400 text-sm">{item.description}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
