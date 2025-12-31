'use client';

import { useState, useEffect } from 'react';

interface Model {
  id: string;
  name: string;
  type: string;
  active: boolean;
  gpu_enabled: boolean;
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

export default function ModelsPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [newModelName, setNewModelName] = useState('');
  const [newModelType, setNewModelType] = useState('embedding');

  const fetchModels = async () => {
    try {
      const res = await fetch(`${API_BASE}/models`);
      if (res.ok) {
        const data = await res.json();
        setModels(data.models || []);
      }
    } catch (e) {
      console.error('Failed to fetch models', e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchModels();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const toggleGpu = async (model: Model) => {
    try {
      const res = await fetch(`${API_BASE}/models/${model.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ gpu_enabled: !model.gpu_enabled })
      });
      if (res.ok) {
        const data = await res.json();
        setModels(models.map(m => m.id === model.id ? data.model : m));
        showMessage('success', `GPU ${model.gpu_enabled ? 'disabled' : 'enabled'} for ${model.name}`);
      }
    } catch (e) {
      showMessage('error', 'Failed to update model');
    }
  };

  const toggleActive = async (model: Model) => {
    try {
      const res = await fetch(`${API_BASE}/models/${model.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active: !model.active })
      });
      if (res.ok) {
        const data = await res.json();
        setModels(models.map(m => m.id === model.id ? data.model : m));
        showMessage('success', `Model ${model.active ? 'deactivated' : 'activated'}`);
      }
    } catch (e) {
      showMessage('error', 'Failed to update model');
    }
  };

  const addModel = async () => {
    if (!newModelName.trim()) return;
    try {
      const res = await fetch(`${API_BASE}/models`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newModelName, type: newModelType })
      });
      if (res.ok) {
        const data = await res.json();
        setModels([...models, data.model]);
        setNewModelName('');
        showMessage('success', `Model ${newModelName} added`);
      }
    } catch (e) {
      showMessage('error', 'Failed to add model');
    }
  };

  const deleteModel = async (model: Model) => {
    if (model.id === 'dense' || model.id === 'sparse') {
      showMessage('error', 'Cannot delete core models');
      return;
    }
    try {
      const res = await fetch(`${API_BASE}/models/${model.id}`, { method: 'DELETE' });
      if (res.ok) {
        setModels(models.filter(m => m.id !== model.id));
        showMessage('success', `Model ${model.name} deleted`);
      }
    } catch (e) {
      showMessage('error', 'Failed to delete model');
    }
  };

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-2">Model Management</h1>
      <p className="text-slate-400 mb-8">View and configure active models</p>

      {message && (
        <div className={`mb-6 p-4 rounded-lg ${
          message.type === 'success' 
            ? 'bg-green-600/20 border border-green-500/30 text-green-400'
            : 'bg-red-600/20 border border-red-500/30 text-red-400'
        }`}>
          {message.type === 'success' ? '✓' : '✗'} {message.text}
        </div>
      )}

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
              <button 
                onClick={() => toggleGpu(model)}
                className="px-4 py-2 bg-slate-700 text-white rounded-lg hover:bg-slate-600 text-sm"
              >
                {model.gpu_enabled ? 'Disable GPU' : 'Enable GPU'}
              </button>
              <button 
                onClick={() => toggleActive(model)}
                className={`px-4 py-2 rounded-lg text-sm ${
                  model.active 
                    ? 'bg-red-600/20 text-red-400 hover:bg-red-600/30' 
                    : 'bg-green-600/20 text-green-400 hover:bg-green-600/30'
                }`}
              >
                {model.active ? 'Deactivate' : 'Activate'}
              </button>
              {model.id !== 'dense' && model.id !== 'sparse' && (
                <button 
                  onClick={() => deleteModel(model)}
                  className="px-4 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 text-sm"
                >
                  Delete
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Add New Model */}
      <div className="mt-8 bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Add New Model</h3>
        <div className="grid grid-cols-2 gap-4">
          <input
            type="text"
            value={newModelName}
            onChange={(e) => setNewModelName(e.target.value)}
            placeholder="Model name (e.g., sentence-transformers/...)"
            className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white"
          />
          <select 
            value={newModelType}
            onChange={(e) => setNewModelType(e.target.value)}
            className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white"
          >
            <option value="embedding">embedding</option>
            <option value="sparse_embedding">sparse_embedding</option>
            <option value="llm">llm</option>
          </select>
        </div>
        <button 
          onClick={addModel}
          className="mt-4 px-6 py-2 bg-primary text-white rounded-lg hover:bg-accent"
        >
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
