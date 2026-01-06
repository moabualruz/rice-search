"use client";

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { Button, Card } from '@/components/ui-elements';
import { Database, Plus, ArrowLeft, Loader2, HardDrive, Settings } from 'lucide-react';

export default function StoreGallery() {
  const [stores, setStores] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadStores();
  }, []);

  const loadStores = async () => {
    try {
      setLoading(true);
      const res = await api.listStores();
      setStores(res);
    } catch (err) {
      setError("Failed to load stores.");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-screen bg-dark p-8">
      {/* Header */}
      <div className="max-w-6xl mx-auto flex items-center justify-between mb-8">
        <div className="flex items-center gap-4">
          <Link href="/" className="text-text-secondary hover:text-primary transition-colors">
            <ArrowLeft size={24} />
          </Link>
          <div>
            <h1 className="text-3xl font-bold text-text flex items-center gap-3">
              <Database className="text-primary" /> Store Gallery
            </h1>
            <p className="text-text-muted mt-1">Manage search indexes and organizations</p>
          </div>
        </div>
        
        <Button onClick={() => alert("Create Store functionality coming next!")}>
          <Plus size={18} className="mr-2" /> New Store
        </Button>
      </div>

      {/* Grid */}
      <div className="max-w-6xl mx-auto">
        {loading ? (
          <div className="flex justify-center p-20 text-text-muted">
            <Loader2 className="animate-spin mr-2" /> Loading stores...
          </div>
        ) : error ? (
           <div className="text-center text-error p-10 bg-dark-secondary rounded-xl border border-border">
             {error}
           </div>
        ) : stores.length === 0 ? (
          <div className="text-center text-text-muted p-20 bg-dark-secondary rounded-xl border border-border border-dashed">
            No stores found. Create one to get started.
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {stores.map((store) => (
              <Card key={store.id} className="hover:border-primary/50 transition-colors group relative overflow-hidden">
                <div className="flex justify-between items-start mb-4">
                  <div className="p-3 bg-dark-tertiary rounded-lg text-primary group-hover:bg-primary/10 group-hover:scale-110 transition-all">
                    <HardDrive size={24} />
                  </div>
                  <span className={`px-2 py-1 text-xs rounded font-medium uppercase tracking-wide
                    ${store.type === 'production' ? 'bg-green-900/30 text-green-400 border border-green-900/50' : 
                      store.type === 'staging' ? 'bg-yellow-900/30 text-yellow-400 border border-yellow-900/50' : 
                      'bg-slate-800 text-slate-400 border border-slate-700'}`}>
                    {store.type}
                  </span>
                </div>
                
                <h3 className="text-xl font-bold text-text mb-2">{store.name}</h3>
                <p className="text-text-muted text-sm line-clamp-2 mb-6 h-10">
                  {store.description || 'No description provided.'}
                </p>
                
                <div className="flex items-center justify-between text-xs text-text-secondary pt-4 border-t border-border">
                   <div className="font-mono">{store.id}</div>
                   {/* Link to detail page (future) */}
                   <Link href={`/stores/${store.id}`}>
                     <Button variant="ghost" size="sm" className="h-8 gap-2 hover:bg-dark-tertiary">
                       Manage <Settings size={14} />
                     </Button>
                   </Link>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    </main>
  );
}
