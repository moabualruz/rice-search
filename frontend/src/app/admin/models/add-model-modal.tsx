"use client";

import { useState } from 'react';
import { Button, Input, Card } from '@/components/ui-elements';
import { Search, Download, Loader2, AlertTriangle, Check } from 'lucide-react';

interface AddModelModalProps {
  onClose: () => void;
  onAdd: (model: any) => Promise<void>;
  apiBase: string;
}

interface SearchResult {
  id: string;
  downloads: number;
  likes: number;
  tags: string[];
}

export default function AddModelModal({ onClose, onAdd, apiBase }: AddModelModalProps) {
  const [query, setQuery] = useState('');
  const [type, setType] = useState('embedding');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState('');
  const [selectedId, setSelectedId] = useState('');

  const handleSearch = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!query) return;

    setSearching(true);
    setError('');
    setResults([]);

    try {
      const res = await fetch(`${apiBase}/models/search?q=${encodeURIComponent(query)}&type=${type}`);
      if (res.ok) {
        const data = await res.json();
        setResults(data.models);
      } else {
        const err = await res.json();
        // Handle Pydantic array errors or string detail
        const errorMsg = Array.isArray(err.detail) 
            ? err.detail.map((e: any) => e.msg).join(', ') 
            : (err.detail || 'Search failed');
        setError(errorMsg);
      }
    } catch (e) {
      setError('Network error');
    } finally {
      setSearching(false);
    }
  };

  const handleInstall = async (modelId: string) => {
    setSelectedId(modelId);
    // Map 'sparse' back to 'sparse_embedding' if needed by system, or keep as is.
    // System seems to use 'sparse_embedding' in badges, but maybe we should unify.
    // For now, let's map it to ensure consistency with existing data.
    const installType = type === 'sparse' ? 'sparse_embedding' : type;
    
    await onAdd({
        name: modelId,
        type: installType,
        active: false,
        gpu_enabled: true 
    });
    // Parent handles closing on success
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <Card className="w-full max-w-2xl max-h-[90vh] flex flex-col p-6 bg-dark border-primary/20 shadow-2xl">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-xl font-bold text-text">Find & Install Models</h2>
          <Button variant="ghost" onClick={onClose}>Close</Button>
        </div>

        {/* Custom Add (Fallback) */}
        <div className="mb-4 flex gap-2 justify-end">
           <button 
             onClick={() => {
                 const id = prompt("Enter manual HuggingFace ID:");
                 if(id) handleInstall(id);
             }}
             className="text-xs text-primary hover:underline"
           >
             Manual Add (Advanced)
           </button>
        </div>

        {/* Search Form */}
        <form onSubmit={handleSearch} className="flex gap-4 mb-6">
          <div className="w-1/4">
             <select 
                className="w-full h-10 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-white focus:ring-2 focus:ring-primary outline-none"
                value={type}
                onChange={e => setType(e.target.value)}
              >
                <option value="embedding">Embedding</option>
                <option value="reranker">Reranker</option>
                <option value="sparse">Sparse (SPLADE)</option>
                <option value="classification">Query Understanding</option>
              </select>
          </div>
          <div className="flex-1">
            <Input 
                autoFocus
                placeholder="Search models (e.g. 'bert', 'jina', 'bge')..." 
                value={query}
                onChange={e => setQuery(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={searching}>
            {searching ? <Loader2 className="animate-spin" /> : <Search />}
          </Button>
        </form>

        {/* Results */}
        <div className="flex-1 overflow-y-auto space-y-2 min-h-[300px]">
           {error && <div className="text-error bg-error/10 p-4 rounded">{error}</div>}
           
           {!searching && results.length === 0 && query && !error && (
               <div className="text-center text-text-muted py-10">No models found referencing &apos;sentence-transformers&apos;.</div>
           )}

           {results.map(model => (
             <div key={model.id} className="flex items-center justify-between p-4 bg-dark-secondary rounded-lg border border-border hover:border-primary/50 transition-colors">
                <div>
                   <div className="font-mono font-bold text-primary mb-1">{model.id}</div>
                   <div className="text-xs text-text-muted flex gap-3">
                      <span className="flex items-center gap-1"><Download size={12}/> {model.downloads.toLocaleString()}</span>
                      <span className="flex items-center gap-1">❤️ {model.likes}</span>
                   </div>
                   <div className="mt-1 flex gap-1 flex-wrap">
                      {model.tags.slice(0, 3).map(t => (
                          <span key={t} className="text-[10px] bg-slate-800 px-1.5 py-0.5 rounded text-slate-400">{t}</span>
                      ))}
                   </div>
                </div>
                <div>
                   <Button 
                     size="sm" 
                     onClick={() => handleInstall(model.id)}
                     disabled={selectedId === model.id}
                   >
                     {selectedId === model.id ? <Loader2 className="animate-spin" size={16}/> : 'Install'}
                   </Button>
                </div>
             </div>
           ))}
        </div>
      </Card>
    </div>
  );
}
