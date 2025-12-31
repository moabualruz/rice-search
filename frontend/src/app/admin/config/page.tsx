'use client';

import { useState } from 'react';

interface ConfigItem {
  key: string;
  value: string | boolean | number;
  type: 'string' | 'boolean' | 'number';
  description: string;
}

const initialConfig: ConfigItem[] = [
  { key: 'SPARSE_ENABLED', value: true, type: 'boolean', description: 'Enable hybrid search with SPLADE' },
  { key: 'SPARSE_MODEL', value: 'naver/splade-v3', type: 'string', description: 'Sparse embedding model' },
  { key: 'EMBEDDING_MODEL', value: 'all-MiniLM-L6-v2', type: 'string', description: 'Dense embedding model' },
  { key: 'RRF_K', value: 60, type: 'number', description: 'RRF fusion parameter' },
  { key: 'AST_PARSING_ENABLED', value: true, type: 'boolean', description: 'Enable Tree-sitter AST parsing' },
  { key: 'MCP_ENABLED', value: false, type: 'boolean', description: 'Enable MCP protocol server' },
  { key: 'MCP_TRANSPORT', value: 'stdio', type: 'string', description: 'MCP transport type' },
  { key: 'MCP_TCP_PORT', value: 9090, type: 'number', description: 'MCP TCP port' },
];

export default function ConfigPage() {
  const [config, setConfig] = useState<ConfigItem[]>(initialConfig);
  const [hasChanges, setHasChanges] = useState(false);

  const updateConfig = (key: string, value: string | boolean | number) => {
    setConfig(config.map(item => 
      item.key === key ? { ...item, value } : item
    ));
    setHasChanges(true);
  };

  const saveConfig = () => {
    alert('Configuration saved! (Restart required for some changes)');
    setHasChanges(false);
  };

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
            onClick={() => setConfig(initialConfig)}
          >
            Reset
          </button>
          <button 
            className={`px-6 py-2 rounded-lg ${
              hasChanges 
                ? 'bg-green-600 text-white hover:bg-green-700' 
                : 'bg-slate-700 text-slate-400 cursor-not-allowed'
            }`}
            onClick={saveConfig}
            disabled={!hasChanges}
          >
            Save Changes
          </button>
        </div>
      </div>

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
            {config.map((item) => (
              <tr key={item.key} className="hover:bg-slate-700/30">
                <td className="px-6 py-4">
                  <code className="text-green-400 bg-slate-900 px-2 py-1 rounded">{item.key}</code>
                </td>
                <td className="px-6 py-4">
                  {item.type === 'boolean' ? (
                    <button
                      onClick={() => updateConfig(item.key, !item.value)}
                      className={`px-4 py-1 rounded-lg text-sm ${
                        item.value 
                          ? 'bg-green-500/20 text-green-400' 
                          : 'bg-slate-700 text-slate-400'
                      }`}
                    >
                      {item.value ? 'Enabled' : 'Disabled'}
                    </button>
                  ) : item.type === 'number' ? (
                    <input
                      type="number"
                      value={item.value as number}
                      onChange={(e) => updateConfig(item.key, parseInt(e.target.value))}
                      className="px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white w-24"
                    />
                  ) : (
                    <input
                      type="text"
                      value={item.value as string}
                      onChange={(e) => updateConfig(item.key, e.target.value)}
                      className="px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white w-64"
                    />
                  )}
                </td>
                <td className="px-6 py-4 text-slate-400 text-sm">{item.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
