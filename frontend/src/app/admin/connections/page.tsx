'use client';

import { useState, useEffect } from 'react';
import { RefreshCw, Monitor, Trash2, Shield, Calendar } from 'lucide-react';
import { api } from '@/lib/api';

interface Connection {
  id: string;
  user_id: string;
  device_name: string;
  version: string;
  last_seen: string;
  ip: string;
}

export default function ConnectionsPage() {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchConnections = async () => {
    try {
      setLoading(true);
      setError(null);
      // We will add this to api.ts shortly
      const res = await api.listConnections(); 
      setConnections(res.connections || []);
    } catch (e) {
      console.error(e);
      setError('Failed to load connections');
    } finally {
      setLoading(false);
    }
  };

  const revokeConnection = async (id: string) => {
    if(!confirm("Are you sure you want to revoke this connection?")) return;
    try {
      await api.deleteConnection(id);
      setConnections(connections.filter(c => c.id !== id));
    } catch (e) {
      console.error(e);
      alert("Failed to revoke connection");
    }
  };

  useEffect(() => {
    fetchConnections();
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white flex items-center gap-3">
             <Monitor className="text-primary" /> Active Connections
          </h1>
          <p className="text-slate-400 mt-1">Manage active CLI sessions and integrations.</p>
        </div>
        <button 
           onClick={fetchConnections}
           className="p-2 bg-slate-800 rounded-lg text-slate-400 hover:text-white transition-colors"
        >
           <RefreshCw size={20} className={loading ? "animate-spin" : ""} />
        </button>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400">
           {error}
        </div>
      )}

      {connections.length === 0 && !loading ? (
        <div className="text-center py-20 bg-slate-800/50 rounded-xl border border-dashed border-slate-700">
           <Monitor size={48} className="mx-auto text-slate-600 mb-4" />
           <h3 className="text-lg font-medium text-slate-300">No active connections</h3>
           <p className="text-slate-500">CLI devices will appear here when they connect.</p>
        </div>
      ) : (
        <div className="grid gap-4">
           {connections.map((conn) => (
             <div key={conn.id} className="bg-slate-800 p-4 rounded-xl border border-slate-700 flex items-center justify-between group hover:border-slate-600 transition-colors">
                <div className="flex items-center gap-4">
                   <div className="p-3 bg-slate-900 rounded-full text-green-400">
                      <Monitor size={20} />
                   </div>
                   <div>
                      <h4 className="font-semibold text-white">{conn.device_name}</h4>
                      <div className="flex items-center gap-3 text-xs text-slate-400 mt-1">
                         <span className="flex items-center gap-1"><Shield size={12}/> {conn.user_id}</span>
                         <span className="flex items-center gap-1"><Calendar size={12}/> {new Date(conn.last_seen).toLocaleString()}</span>
                         <span className="bg-slate-700 px-1.5 py-0.5 rounded text-slate-300 font-mono">v{conn.version}</span>
                         <span>{conn.ip}</span>
                      </div>
                   </div>
                </div>
                <button 
                  onClick={() => revokeConnection(conn.id)}
                  className="p-2 text-slate-500 hover:text-red-400 hover:bg-slate-700/50 rounded-lg transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100"
                  title="Revoke Access"
                >
                  <Trash2 size={20} />
                </button>
             </div>
           ))}
        </div>
      )}
    </div>
  );
}
