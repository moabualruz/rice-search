"use client";

import { useState } from 'react';
import Image from 'next/image';
import { Button, Input, Card } from '@/components/ui-elements';
import { Search, Sparkles, Database, FileText, ChevronRight } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import { api, type SearchResult } from '@/lib/api';

export default function Home() {
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<'search' | 'rag'>('rag');
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [answer, setAnswer] = useState<string | null>(null);

  const handleSearch = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setAnswer(null);
    setResults([]);

    try {
      const res = await api.search(query, mode);
      if (mode === 'rag') {
        setAnswer(res.answer || "No answer generated.");
        setResults(res.sources || []);
      } else {
        setResults(res.results || []);
      }
    } catch (err) {
      console.error(err);
      setAnswer("Error occurred while searching.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="flex min-h-screen flex-col items-center px-4 pt-24 pb-12">
      {/* Hero */}
      <div className="w-full max-w-4xl space-y-8 text-center">
        <div className="space-y-4">
          <div className="flex items-center justify-center gap-4 mb-4">
            <Image 
              src="/logo.svg" 
              alt="Rice Search" 
              width={64} 
              height={64}
              className="rounded-xl"
            />
          </div>
          <h1 className="text-5xl font-extrabold tracking-tight sm:text-7xl text-primary">
            Rice Search
          </h1>
          <p className="text-lg text-slate-400 max-w-2xl mx-auto">
            Enterprise-grade Neural Search & Retrieval Augmented Generation.
          </p>
        </div>

        {/* Search Bar */}
        <form onSubmit={handleSearch} className="relative max-w-2xl mx-auto">
          <div className="relative group">
            <div className="absolute -inset-0.5 bg-gradient-to-r from-primary to-accent rounded-xl opacity-50 group-hover:opacity-100 transition duration-200 blur"></div>
            <div className="relative flex items-center bg-slate-900 rounded-xl p-2 gap-2">
               <div className="pl-3 text-slate-400">
                  {mode === 'rag' ? <Sparkles size={20} /> : <Search size={20} />}
               </div>
              <Input 
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={mode === 'rag' ? "Ask anything..." : "Search documents..."}
                className="border-0 bg-transparent focus:ring-0 text-lg h-12"
              />
              <div className="flex bg-slate-800 rounded-lg p-1 gap-1">
                <button
                   type="button"
                   onClick={() => setMode('search')}
                   className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${mode === 'search' ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white'}`}
                >
                  Search
                </button>
                <button
                   type="button"
                   onClick={() => setMode('rag')}
                   className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${mode === 'rag' ? 'bg-purple-500 text-white' : 'text-slate-400 hover:text-white'}`}
                >
                  Ask AI
                </button>
              </div>
              <Button type="submit" size="lg" loading={loading} className="rounded-lg h-12 w-12 p-0 flex items-center justify-center">
                <ChevronRight size={24} />
              </Button>
            </div>
          </div>
        </form>

        {/* Results */}
        <div className="w-full max-w-3xl mx-auto text-left space-y-6 mt-12 pb-20">
          
          {/* AI Answer */}
          {answer && (
            <Card className="border-purple-500/20 bg-purple-500/5">
              <div className="flex items-center gap-2 mb-4 text-purple-400">
                <Sparkles size={18} />
                <span className="text-sm font-semibold uppercase tracking-wider">AI Answer</span>
              </div>
              <div className="prose prose-invert max-w-none text-slate-200">
                <ReactMarkdown>{answer}</ReactMarkdown>
              </div>
            </Card>
          )}

          {/* Sources / Hits */}
          {results.length > 0 && (
             <div className="space-y-4">
                <div className="flex items-center gap-2 text-slate-500 px-1">
                  <Database size={16} />
                  <span className="text-xs font-medium uppercase tracking-wider">
                    {mode === 'rag' ? 'Sources' : 'Results'}
                  </span>
                </div>
                {results.map((hit, i) => (
                  <Card key={i} className="hover:border-slate-700 transition-colors group">
                    <div className="flex items-start gap-4">
                      <div className="mt-1 p-2 bg-slate-800 rounded-lg text-indigo-400 group-hover:text-indigo-300">
                        <FileText size={20} />
                      </div>
                      <div className="flex-1 space-y-2">
                        <div className="flex items-center justify-between">
                           <h3 className="font-medium text-slate-200 text-sm">
                             {hit.metadata?.filename || 'Unknown Document'}
                           </h3>
                           <span className="text-xs text-slate-500 px-2 py-1 bg-slate-950 rounded-full">
                             {(hit.score * 100).toFixed(1)}% match
                           </span>
                        </div>
                        <p className="text-slate-400 text-sm line-clamp-2">
                          {hit.text}
                        </p>
                      </div>
                    </div>
                  </Card>
                ))}
             </div>
          )}

        </div>
      </div>
      
      {/* Footer Link to Admin */}
      <div className="fixed bottom-4 right-4">
         <a href="/admin" className="text-xs text-slate-600 hover:text-slate-400 transition-colors">Admin Dashboard &rarr;</a>
      </div>
    </main>
  );
}
