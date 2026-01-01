"use client";

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { Button, Input, Card } from '@/components/ui-elements';
import { ArrowLeft, Plus, Cpu, Zap, Trash2, AlertTriangle, Download, Server } from 'lucide-react';

const API_BASE = 'http://localhost:8000/api/v1/admin/public';

interface Model {
  id: string;
  name: string;
  type: 'embedding' | 'reranker' | 'sparse_embedding' | 'classification';
  active: boolean;
  gpu_enabled: boolean;
}

export default function AdminModels() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  
  // New model form
  const [newModel, setNewModel] = useState({
    name: '',
    type: 'embedding',
    gpu_enabled: true
  });

  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadModels();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const loadModels = async () => {
    try {
      const res = await fetch(`${API_BASE}/models`);
      if (res.ok) {
        const data = await res.json();
        setModels(data.models);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newModel.name) return;

    try {
      const res = await fetch(`${API_BASE}/models`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newModel.name, // HuggingFace ID
          type: newModel.type,
          active: false, // Inactive by default until verified
          gpu_enabled: newModel.gpu_enabled
        })
      });

      if (res.ok) {
        showMessage('success', 'Model added successfully. Restart required to download weights.');
        setShowAdd(false);
        setNewModel({ name: '', type: 'embedding', gpu_enabled: true });
        loadModels();
      } else {
        const err = await res.json();
        showMessage('error', err.detail || 'Failed to add model');
      }
    } catch (e) {
      showMessage('error', 'Failed to add model');
    }
  };

  const updateModel = async (id: string, updates: Partial<Model>) => {
    try {
      const res = await fetch(`${API_BASE}/models/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates)
      });
      
      if (res.ok) {
         showMessage('success', 'Model updated. Restart may be required.');
         // Optimistic update
         setModels(models.map(m => m.id === id ? { ...m, ...updates } : m));
      } else {
         showMessage('error', 'Failed to update model');
      }
    } catch (e) {
       showMessage('error', 'Failed to update model');
    }
  };

  const deleteModel = async (id: string) => {
     if (!confirm(`Delete model ${id}? This cannot be undone.`)) return;
     
     try {
       const res = await fetch(`${API_BASE}/models/${id}`, { method: 'DELETE' });
       if (res.ok) {
         showMessage('success', 'Model deleted');
         setModels(models.filter(m => m.id !== id));
       } else {
         const err = await res.json();
         showMessage('error', err.detail || 'Failed to delete');
       }
     } catch (e) {
       showMessage('error', 'Failed to delete');
     }
  };

  return (
    <main className="min-h-screen bg-dark p-8">
      {/* Header */}
      <div className="max-w-6xl mx-auto mb-8">
        <div className="flex items-center gap-4 mb-4">
          <Link href="/admin" className="text-text-secondary hover:text-primary transition-colors">
            <ArrowLeft size={24} />
          </Link>
          <h1 className="text-3xl font-bold text-text flex items-center gap-3">
             <Server className="text-primary" /> Model Registry
          </h1>
        </div>
        <p className="text-text-muted">Manage AI models for Embedding, Reranking, and Classification.</p>
      </div>

      <div className="max-w-6xl mx-auto">
        {message && (
          <div className={`mb-6 p-4 rounded-lg bg-dark-secondary border ${
            message.type === 'success' ? 'border-green-500/30 text-green-400' : 'border-red-500/30 text-red-400'
          }`}>
             {message.text}
          </div>
        )}

        <div className="flex justify-between items-center mb-6">
           <div className="text-sm text-text-muted">
              Active Models: {models.filter(m => m.active).length} / {models.length}
           </div>
           <Button onClick={() => setShowAdd(!showAdd)}>
             <Plus size={18} className="mr-2" /> Add Model
           </Button>
        </div>

        {/* Add Form */}
        {showAdd && (
          <Card className="mb-8 border-primary/30">
            <h3 className="text-lg font-bold text-text mb-4">Add New Model</h3>
            <form onSubmit={handleAdd} className="space-y-4">
               <div>
                 <label className="block text-sm text-text-secondary mb-1">HuggingFace Model ID</label>
                 <Input 
                   value={newModel.name} 
                   onChange={e => setNewModel({...newModel, name: e.target.value})}
                   placeholder="e.g. jinaai/jina-embeddings-v2-base-code" 
                   required
                 />
               </div>
               
               <div className="flex gap-4">
                 <div className="flex-1">
                   <label className="block text-sm text-text-secondary mb-1">Type</label>
                   <select 
                     className="w-full h-10 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-white focus:ring-2 focus:ring-primary outline-none"
                     value={newModel.type}
                     onChange={e => setNewModel({...newModel, type: e.target.value as any})}
                   >
                     <option value="embedding">Embedding</option>
                     <option value="reranker">Reranker</option>
                     <option value="sparse_embedding">Sparse (SPLADE)</option>
                     <option value="classification">Classification</option>
                   </select>
                 </div>
                 
                 <div className="flex items-center pt-6">
                    <label className="flex items-center gap-2 text-text cursor-pointer">
                      <input 
                        type="checkbox" 
                        checked={newModel.gpu_enabled}
                        onChange={e => setNewModel({...newModel, gpu_enabled: e.target.checked})}
                        className="w-4 h-4 rounded border-slate-700 bg-slate-900 text-primary focus:ring-primary"
                      />
                      Enable GPU
                    </label>
                 </div>
               </div>

               <div className="flex justify-end gap-2 pt-2">
                 <Button type="button" variant="ghost" onClick={() => setShowAdd(false)}>Cancel</Button>
                 <Button type="submit">Add to Registry</Button>
               </div>
            </form>
          </Card>
        )}

        {/* List */}
        {loading ? (
          <div className="text-center p-20 text-text-muted">Loading...</div>
        ) : (
          <div className="grid gap-4">
            {models.map(model => (
               <Card key={model.id} className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                  <div className="flex-1">
                     <div className="flex items-center gap-3 mb-1">
                        <span className={`px-2 py-0.5 rounded text-xs uppercase font-bold tracking-wide
                          ${model.type === 'embedding' ? 'bg-blue-900/30 text-blue-400' :
                            model.type === 'reranker' ? 'bg-purple-900/30 text-purple-400' :
                            model.type === 'sparse_embedding' ? 'bg-orange-900/30 text-orange-400' :
                            'bg-gray-800 text-gray-400'
                          }`}>
                          {model.type}
                        </span>
                        <h3 className="text-lg font-mono text-text font-bold">{model.name}</h3>
                        {model.active && <span className="text-xs text-green-400 border border-green-900/50 px-2 rounded-full">Active</span>}
                     </div>
                     <div className="text-sm text-text-muted font-mono pl-1">ID: {model.id}</div>
                  </div>

                  <div className="flex items-center gap-6">
                     {/* Controls */}
                     
                     {/* GPU Toggle */}
                     <div className="flex flex-col items-center gap-1">
                        <span className="text-xs text-text-muted uppercase">Hardware</span>
                        <button 
                          onClick={() => updateModel(model.id, { gpu_enabled: !model.gpu_enabled })}
                          className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border transition-all ${
                             model.gpu_enabled 
                               ? 'bg-green-900/20 border-green-800 text-green-400 hover:bg-green-900/40' 
                               : 'bg-slate-800 border-slate-700 text-slate-500 hover:text-slate-300'
                          }`}
                          title={model.gpu_enabled ? "Running on GPU" : "Running on CPU"}
                        >
                           {model.gpu_enabled ? <Zap size={14} fill="currentColor" /> : <Cpu size={14} />}
                           <span className="text-xs font-bold">{model.gpu_enabled ? 'GPU' : 'CPU'}</span>
                        </button>
                     </div>

                     {/* Active Toggle */}
                     <div className="flex flex-col items-center gap-1">
                        <span className="text-xs text-text-muted uppercase">Status</span>
                        <label className="relative inline-flex items-center cursor-pointer">
                          <input 
                            type="checkbox" 
                            className="sr-only peer"
                            checked={model.active}
                            onChange={(e) => updateModel(model.id, { active: e.target.checked })}
                          />
                          <div className="w-11 h-6 bg-slate-700 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-primary rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                        </label>
                     </div>
                     
                     {/* Delete */}
                     <div className="flex flex-col items-center gap-1 pl-4 border-l border-slate-700">
                        <span className="text-xs text-text-muted uppercase">Action</span>
                        <Button 
                          variant="ghost" 
                          size="sm" 
                          className="text-text-muted hover:text-error hover:bg-error/10"
                          onClick={() => deleteModel(model.id)}
                          disabled={['dense', 'sparse'].includes(model.id)} // Protect core models
                        >
                           <Trash2 size={16} />
                        </Button>
                     </div>
                  </div>
               </Card>
            ))}
          </div>
        )}
        
        <div className="mt-8 p-4 bg-dark-secondary rounded-xl border border-border flex items-start gap-3">
           <AlertTriangle className="text-warning shrink-0" />
           <div>
              <h4 className="text-text font-bold">Restart Required</h4>
              <p className="text-sm text-text-muted">
                Changes to model configuration (Active/GPU) require a system restart to take full effect. 
                Weights will be downloaded on first startup.
              </p>
           </div>
        </div>
      </div>
    </main>
  );
}
