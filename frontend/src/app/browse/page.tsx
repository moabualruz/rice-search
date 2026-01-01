"use client";

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { Input, Button, Card } from '@/components/ui-elements';
import { FileText, Folder, Search, ArrowLeft, Loader2, Code, AlertCircle } from 'lucide-react';

export default function FileExplorer() {
  // State
  const [files, setFiles] = useState<string[]>([]);
  const [filteredFiles, setFilteredFiles] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState<{ content: string; language: string } | null>(null);
  const [contentLoading, setContentLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Initial load
  useEffect(() => {
    loadFiles();
  }, []);

  // Filter effect
  useEffect(() => {
    if (!filter.trim()) {
      setFilteredFiles(files);
      return;
    }
    const lowerFilter = filter.toLowerCase();
    setFilteredFiles(files.filter(f => f.toLowerCase().includes(lowerFilter)));
  }, [filter, files]);

  // Actions
  const loadFiles = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await api.listFiles();
      setFiles(res.files);
      setFilteredFiles(res.files);
    } catch (err) {
      setError("Failed to load file list.");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleFileClick = async (path: string) => {
    setSelectedFile(path);
    setContentLoading(true);
    setFileContent(null);
    try {
      const res = await api.getFileContent(path);
      setFileContent(res);
    } catch (err) {
      console.error(err);
      setFileContent({ content: "Error loading file content.", language: "text" });
    } finally {
      setContentLoading(false);
    }
  };

  return (
    <main className="flex min-h-screen flex-col bg-dark">
      {/* Header */}
      <header className="border-b border-border bg-dark-secondary p-4 flex items-center gap-4 sticky top-0 z-10">
        <Link href="/" className="text-text-secondary hover:text-primary transition-colors">
          <ArrowLeft size={20} />
        </Link>
        <div className="flex items-center gap-2">
          <Folder className="text-primary" size={24} />
          <h1 className="text-xl font-bold text-text">File Explorer</h1>
        </div>
        <div className="flex-1 max-w-xl mx-auto">
           <div className="relative">
             <Search className="absolute left-3 top-2.5 text-text-muted" size={18} />
             <Input 
               placeholder="Filter by filename..." 
               value={filter}
               onChange={(e) => setFilter(e.target.value)}
               className="pl-10 bg-dark-tertiary border-border h-10"
             />
           </div>
        </div>
        <div className="text-sm text-text-muted">
          {loading ? 'Loading...' : `${files.length} files`}
        </div>
      </header>

      {/* Content */}
      <div className="flex-1 flex overflow-hidden h-[calc(100vh-73px)]">
        
        {/* Left Sidebar: File List */}
        <div className="w-1/3 min-w-[300px] border-r border-border overflow-y-auto bg-dark-secondary flex flex-col">
          {loading ? (
            <div className="flex items-center justify-center h-full text-text-muted">
              <Loader2 className="animate-spin mr-2" /> Loading index...
            </div>
          ) : error ? (
            <div className="p-8 text-center text-error flex flex-col items-center">
               <AlertCircle size={32} className="mb-2" />
               <p>{error}</p>
               <Button onClick={loadFiles} variant="outline" size="sm" className="mt-4">Retry</Button>
            </div>
          ) : filteredFiles.length === 0 ? (
            <div className="p-8 text-center text-text-muted">
              No files found.
            </div>
          ) : (
            <div className="flex flex-col">
              {filteredFiles.map((file) => (
                <button
                  key={file}
                  onClick={() => handleFileClick(file)}
                  className={`flex items-center gap-3 px-4 py-3 text-sm text-left border-b border-border/50 transition-colors
                    ${selectedFile === file 
                      ? 'bg-primary/10 text-primary border-l-4 border-l-primary' 
                      : 'text-text-secondary hover:bg-dark-tertiary hover:text-text border-l-4 border-l-transparent'
                    }`}
                >
                  <FileText size={16} className="shrink-0" />
                  <span className="truncate font-mono text-xs">{file}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Right Pane: Content */}
        <div className="flex-1 overflow-y-auto bg-dark p-6">
          {selectedFile ? (
            <div className="max-w-5xl mx-auto space-y-4">
               <div className="flex items-center justify-between pb-4 border-b border-border">
                  <h2 className="text-lg font-mono text-text flex items-center gap-2">
                    <Code size={18} className="text-accent" />
                    {selectedFile}
                  </h2>
                  {fileContent && (
                    <span className="text-xs px-2 py-1 bg-dark-tertiary rounded text-text-muted uppercase">
                      {fileContent.language}
                    </span>
                  )}
               </div>

               {contentLoading ? (
                 <div className="flex items-center justify-center p-20 text-text-muted">
                    <Loader2 className="animate-spin w-8 h-8" />
                 </div>
               ) : (
                 <Card className="p-0 overflow-hidden border-border bg-dark-secondary">
                   <div className="overflow-x-auto p-4">
                     <pre className="font-mono text-sm text-text-secondary whitespace-pre">
                       <code>{fileContent?.content}</code>
                     </pre>
                   </div>
                 </Card>
               )}
            </div>
          ) : (
             <div className="flex flex-col items-center justify-center h-full text-text-muted opacity-50">
                <Search size={64} className="mb-4" />
                <p className="text-xl">Select a file to view content</p>
             </div>
          )}
        </div>

      </div>
    </main>
  );
}
