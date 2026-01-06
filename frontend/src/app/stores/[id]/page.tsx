"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { Button, Card, Input } from "@/components/ui-elements";
import { ArrowLeft, File as FileIcon, Search, Trash2, Database, Shield, Server } from "lucide-react";

type Store = {
  id: string;
  name: string;
  description: string;
  org_id: string;
  type: "prod" | "staging" | "dev";
  doc_count: number;
  created_at?: string;
};

export default function StoreDetail() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;

  const [store, setStore] = useState<Store | null>(null);
  const [files, setFiles] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    fetchData();
  }, [id]);

  const fetchData = async () => {
    try {
      setLoading(true);
      const storeData = await api.getStore(id);
      setStore(storeData);
      
      // Fetch files for this store (org_id)
      // Assuming store.org_id corresponds to the org_id used in file listing
      // If store.id is the org_id, use that. Based on endpoint it seems id matches.
      const filesData = await api.listFiles(undefined, storeData.org_id || storeData.id);
      setFiles(filesData.files);
    } catch (err) {
      setError("Failed to load store details");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!store) return;
    try {
      const data = await api.listFiles(searchQuery, store.org_id || store.id);
      setFiles(data.files);
    } catch (err) {
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!confirm("Are you sure you want to delete this store? This action cannot be undone.")) return;
    
    try {
      setIsDeleting(true);
      await api.deleteStore(id);
      router.push("/stores");
    } catch (err) {
      alert("Failed to delete store");
      setIsDeleting(false);
    }
  };

  if (loading) {
    return (
      <main className="min-h-screen bg-dark p-8 flex items-center justify-center">
        <div className="text-primary animate-pulse">Loading store details...</div>
      </main>
    );
  }

  if (error || !store) {
    return (
      <main className="min-h-screen bg-dark p-8 flex items-center justify-center flex-col gap-4">
        <div className="text-red-400">{error || "Store not found"}</div>
        <Link href="/stores">
          <Button variant="secondary">Back to Stores</Button>
        </Link>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-dark p-8">
      {/* Header / Nav */}
      <div className="max-w-6xl mx-auto mb-8">
        <Link href="/stores" className="inline-flex items-center text-slate-400 hover:text-white mb-6 transition-colors">
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Gallery
        </Link>

        <div className="flex flex-col md:flex-row md:items-start justify-between gap-6">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-3xl font-bold text-white tracking-tight">{store.name}</h1>
              <span className={`px-2 py-0.5 rounded text-xs uppercase font-medium border ${
                store.type === 'prod' ? 'bg-green-900/30 text-green-400 border-green-800' :
                store.type === 'staging' ? 'bg-yellow-900/30 text-yellow-400 border-yellow-800' :
                'bg-blue-900/30 text-blue-400 border-blue-800'
              }`}>
                {store.type}
              </span>
            </div>
            <p className="text-slate-400 max-w-2xl">{store.description}</p>
            
            <div className="flex items-center gap-6 mt-4 text-sm text-slate-500 font-mono">
              <div className="flex items-center gap-2">
                <Shield className="w-4 h-4" />
                ID: {store.id}
              </div>
              <div className="flex items-center gap-2">
                <Server className="w-4 h-4" />
                Org: {store.org_id}
              </div>
              <div className="flex items-center gap-2">
                <Database className="w-4 h-4" />
                {store.doc_count} Documents
              </div>
            </div>
          </div>

          <Button 
            variant="secondary" 
            onClick={handleDelete} 
            disabled={isDeleting}
            className="shrink-0 bg-red-900/20 text-red-500 hover:bg-red-900/40 border-red-900/50"
          >
            {isDeleting ? "Deleting..." : "Delete Store"}
            <Trash2 className="w-4 h-4 ml-2" />
          </Button>
        </div>
      </div>

      {/* Content Area */}
      <div className="max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-4 gap-8">
        
        {/* Sidebar: Stats or Filters */}
        <div className="hidden lg:block space-y-6">
          <Card className="p-4 bg-dark-secondary border-border">
            <h3 className="text-sm font-semibold text-slate-400 mb-4 uppercase tracking-wider">Store Stats</h3>
            <div className="space-y-4">
              <div>
                <div className="text-2xl font-mono text-white">{files.length}</div>
                <div className="text-xs text-slate-500">Indexed Files</div>
              </div>
              <div>
                <div className="text-2xl font-mono text-white">{store.doc_count}</div>
                <div className="text-xs text-slate-500">Total Chunks</div>
              </div>
            </div>
          </Card>
        </div>

        {/* Main: File Browser */}
        <div className="lg:col-span-3 space-y-6">
          <form onSubmit={handleSearch} className="flex gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
              <Input 
                placeholder="Search files in this store..." 
                className="pl-9 w-full bg-dark-secondary border-slate-700 focus:border-primary"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
            <Button type="submit">Search</Button>
          </form>

          <div className="bg-dark-secondary rounded-xl border border-border overflow-hidden">
             <div className="p-4 border-b border-border bg-slate-800/50 flex items-center justify-between">
                <h3 className="font-medium text-white">Files</h3>
                <span className="text-xs text-slate-500">{files.length} results</span>
             </div>
             
             {files.length === 0 ? (
               <div className="p-12 text-center text-slate-500">
                 <FileIcon className="w-12 h-12 mx-auto mb-3 opacity-20" />
                 <p>No files found in this store.</p>
               </div>
             ) : (
               <div className="divide-y divide-border">
                 {files.map((file, i) => (
                   <div key={i} className="p-4 hover:bg-slate-800/50 transition-colors flex items-center gap-3 group cursor-default">
                     <FileIcon className="w-4 h-4 text-slate-400 group-hover:text-primary transition-colors" />
                     <span className="text-sm text-slate-300 font-mono truncate flex-1">{file}</span>
                     {/* Could add 'View' button here later linking to file explorer with context */}
                   </div>
                 ))}
               </div>
             )}
          </div>
        </div>
      </div>
    </main>
  );
}
