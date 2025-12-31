"use client";

import { useState } from 'react';
import { Button, Input, Card } from '@/components/ui-elements';
import { Upload, FileText, CheckCircle2, AlertCircle } from 'lucide-react';
import { api } from '@/lib/api';
import { useSession, signIn, signOut } from "next-auth/react";
import { redirect } from 'next/navigation';

export default function AdminPage() {
  const { data: session, status: sessionStatus } = useSession();
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle');

  if (sessionStatus === "loading") {
    return <div className="min-h-screen flex items-center justify-center">Loading...</div>;
  }

  if (sessionStatus === "unauthenticated") {
     return (
        <div className="min-h-screen flex items-center justify-center bg-slate-900">
           <Card className="text-center p-10 space-y-6">
              <h1 className="text-2xl font-bold">Admin Access Required</h1>
              <p className="text-slate-400">You must be logged in to manage documents.</p>
              <Button onClick={() => signIn("keycloak")}>Login with Keycloak</Button>
           </Card>
        </div>
     )
  }

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file || !session?.accessToken) return;

    setUploading(true);
    setStatus('idle');

    try {
      await api.ingest(file, session.accessToken as string);
      setStatus('success');
      setFile(null);
    } catch (err) {
      console.error(err);
      setStatus('error');
    } finally {
      setUploading(false);
    }
  };

  return (
    <main className="min-h-screen p-8 max-w-4xl mx-auto space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Admin Dashboard</h1>
        <div className="flex items-center gap-4">
            <span className="text-sm text-slate-400">Logged in as {session?.user?.name || session?.user?.email}</span>
            <Button variant="outline" size="sm" onClick={() => signOut()}>Logout</Button>
            <a href="/" className="text-sm text-slate-400 hover:text-white">Back to Search</a>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Ingestion Card */}
        <Card className="space-y-6">
          <div className="flex items-center gap-3 border-b border-slate-800 pb-4">
            <div className="p-2 bg-indigo-500/10 text-indigo-400 rounded-lg">
              <Upload size={20} />
            </div>
            <div>
              <h2 className="font-semibold text-lg">Ingest Documents</h2>
              <p className="text-sm text-slate-500">Upload PDF, TXT, or MD files</p>
            </div>
          </div>

          <form onSubmit={handleUpload} className="space-y-4">
            <div className={`
              border-2 border-dashed rounded-xl p-8 flex flex-col items-center justify-center text-center transition-colors
              ${file ? 'border-indigo-500/50 bg-indigo-500/5' : 'border-slate-800 hover:border-slate-700'}
            `}>
              <input 
                type="file" 
                id="file-upload" 
                className="hidden" 
                onChange={(e) => setFile(e.target.files?.[0] || null)}
              />
              <label htmlFor="file-upload" className="cursor-pointer space-y-2 w-full">
                <div className="mx-auto w-12 h-12 bg-slate-800 rounded-full flex items-center justify-center text-slate-400">
                  {file ? <FileText size={20} /> : <Upload size={20} />}
                </div>
                <div className="text-sm">
                  {file ? (
                    <span className="text-indigo-400 font-medium">{file.name}</span>
                  ) : (
                    <span className="text-slate-400">Click to select file</span>
                  )}
                </div>
              </label>
            </div>
            
            <div className="flex justify-end">
              <Button type="submit" disabled={!file} loading={uploading}>
                Upload Document
              </Button>
            </div>
          </form>

          {status === 'success' && (
            <div className="flex items-center gap-2 p-3 bg-green-500/10 text-green-400 rounded-lg text-sm">
              <CheckCircle2 size={16} />
              Document uploaded successfully! Processing in background.
            </div>
          )}
           {status === 'error' && (
            <div className="flex items-center gap-2 p-3 bg-red-500/10 text-red-400 rounded-lg text-sm">
              <AlertCircle size={16} />
              Upload failed. Check backend logs.
            </div>
          )}
        </Card>

        {/* System Status Placeholder */}
        <Card className="space-y-6 opacity-50 pointer-events-none">
           <div className="flex items-center gap-3 border-b border-slate-800 pb-4">
            <div className="p-2 bg-purple-500/10 text-purple-400 rounded-lg">
              <FileText size={20} />
            </div>
            <div>
              <h2 className="font-semibold text-lg">System Health</h2>
              <p className="text-sm text-slate-500">Monitor Indexing & Workers</p>
            </div>
          </div>
          <div className="h-40 flex items-center justify-center text-slate-600">
            Coming Soon
          </div>
        </Card>
      </div>
    </main>
  );
}
