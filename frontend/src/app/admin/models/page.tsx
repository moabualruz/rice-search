'use client';

import { useState, useEffect } from 'react';

interface Model {
  id: string;
  name: string;
  type: string;
  active: boolean;
  gpu_enabled: boolean;
}

export default function ModelsPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Simulated data - would fetch from /api/v1/admin/models
    setModels([
      { id: 'dense', name: 'all-MiniLM-L6-v2', type: 'embedding', active: true, gpu_enabled: false },
      { id: 'sparse', name: 'naver/splade-v3', type: 'sparse_embedding', active: true, gpu_enabled: true },
      { id: 'llm', name: 'gpt-3.5-turbo', type: 'llm', active: true, gpu_enabled: false },
    ]);
    setLoading(false);
  }, []);

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-2">Model Management</h1>
      <p className="text-slate-400 mb-8">View and configure active models</p>

      <div className="grid gap-6">
        {models.map((model) => (
          <div key={model.id} className="bg-slate-800 rounded-xl p-6 border border-slate-700">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-xl font-semibold text-white">{model.name}</h3>
                <p className="text-slate-400 text-sm">{model.type} • ID: {model.id}</p>
              </div>
              <div className="flex items-center gap-4">
                <StatusBadge label="Active" enabled={model.active} />
                <StatusBadge label="GPU" enabled={model.gpu_enabled} />
              </div>
            </div>
            
            <div className="mt-4 flex gap-3">
              <button className="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 text-sm">
                Configure
              </button>
              <button className="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 text-sm">
                {model.gpu_enabled ? 'Disable GPU' : 'Enable GPU'}
              </button>
              <button className="px-4 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 text-sm">
                Deactivate
              </button>
            </div>
          </div>
        ))}
      </div>

      <div className="mt-8 bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Add New Model</h3>
        <div className="grid grid-cols-2 gap-4">
          <input
            type="text"
            placeholder="Model name (e.g., sentence-transformers/...)"
            className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white"
          />
          <select className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white">
            <option>embedding</option>
            <option>sparse_embedding</option>
            <option>llm</option>
          </select>
        </div>
        <button className="mt-4 px-6 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
          Add Model
        </button>
      </div>
    </div>
  );
}

function StatusBadge({ label, enabled }: { label: string; enabled: boolean }) {
  return (
    <span className={`px-3 py-1 rounded-full text-sm ${
      enabled 
        ? 'bg-green-500/20 text-green-400 border border-green-500/30'
        : 'bg-slate-700 text-slate-400 border border-slate-600'
    }`}>
      {enabled ? '✓' : '○'} {label}
    </span>
  );
}
