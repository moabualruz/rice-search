"use client";

import { useEffect, useState, useMemo } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { Input, Button, Card } from '@/components/ui-elements';
import { FileText, Folder, FolderOpen, Search, ArrowLeft, Loader2, Code, AlertCircle, ChevronRight, ChevronDown } from 'lucide-react';

type TreeNode = {
  name: string;
  path: string;
  type: 'file' | 'folder';
  children?: TreeNode[];
};

export default function FileExplorer() {
  // State
  const [files, setFiles] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState<{ content: string; language: string } | null>(null);
  const [contentLoading, setContentLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set());

  // Initial load
  useEffect(() => {
    loadFiles();
  }, []);

  // Actions
  const loadFiles = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await api.listFiles();
      setFiles(res.files);
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

  const toggleFolder = (path: string) => {
    const next = new Set(expandedFolders);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    setExpandedFolders(next);
  };

  // Transform flat list to tree
  const fileTree = useMemo(() => {
    const root: TreeNode[] = [];
    
    // Filter first
    const filtered = filter 
      ? files.filter(f => f.toLowerCase().includes(filter.toLowerCase()))
      : files;

    filtered.forEach(path => {
      const parts = path.split('/');
      let currentLevel = root;
      let currentPath = '';

      parts.forEach((part, index) => {
        currentPath = currentPath ? `${currentPath}/${part}` : part;
        const isFile = index === parts.length - 1;
        
        const existingNode = currentLevel.find(n => n.name === part);
        
        if (existingNode) {
          if (!isFile) {
            currentLevel = existingNode.children!;
          }
        } else {
          const newNode: TreeNode = {
            name: part,
            path: currentPath,
            type: isFile ? 'file' : 'folder',
            children: isFile ? undefined : []
          };
          currentLevel.push(newNode);
          if (!isFile) {
            currentLevel = newNode.children!;
          }
        }
      });
    });

    // Recursive sort: folders first, then files, both alphabetical
    const sortNodes = (nodes: TreeNode[]) => {
      nodes.sort((a, b) => {
        if (a.type === b.type) return a.name.localeCompare(b.name);
        return a.type === 'folder' ? -1 : 1;
      });
      nodes.forEach(n => {
        if (n.children) sortNodes(n.children);
      });
    };
    
    sortNodes(root);
    return root;
  }, [files, filter]);

  // Keep filtered folders expanded
  useEffect(() => {
    if (filter) {
        // Simple heuristic: if searching, expand everything to show matches
        // Or strictly: we don't strictly need to track expanded state for filter view
        // But a cleaner UX might be to auto-expand all parents of matches. 
        // For now, let's just assume search result view is fully expanded or flat?
        // Let's stick to tree structure even in search, but maybe auto expand all?
        // Implementing auto-expand all for now on filter change
        const allPaths = new Set<string>();
        const traverse = (nodes: TreeNode[]) => {
            nodes.forEach(n => {
                if (n.type === 'folder') {
                    allPaths.add(n.path);
                    if (n.children) traverse(n.children);
                }
            });
        };
        traverse(fileTree);
        setExpandedFolders(allPaths);
    }
  }, [filter, fileTree]);


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
        
        {/* Left Sidebar: File Tree */}
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
          ) : files.length === 0 ? (
            <div className="p-8 text-center text-text-muted">
              No files found.
            </div>
          ) : (
            <div className="p-2">
              <FileTree 
                nodes={fileTree} 
                selectedFile={selectedFile} 
                onSelect={handleFileClick} 
                expandedFolders={expandedFolders}
                onToggleFolder={toggleFolder}
              />
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

// Recursive Tree Component
function FileTree({ 
  nodes, 
  selectedFile, 
  onSelect, 
  expandedFolders, 
  onToggleFolder,
  level = 0
}: { 
  nodes: TreeNode[]; 
  selectedFile: string | null; 
  onSelect: (path: string) => void;
  expandedFolders: Set<string>;
  onToggleFolder: (path: string) => void;
  level?: number;
}) {
  return (
    <div className="flex flex-col">
      {nodes.map((node) => (
        <div key={node.path}>
          {node.type === 'folder' ? (
            <div>
              <button
                onClick={() => onToggleFolder(node.path)}
                className={`flex items-center gap-2 w-full text-left py-1 px-2 text-sm text-text-secondary hover:bg-dark-tertiary rounded
                  ${expandedFolders.has(node.path) ? 'text-text' : ''}`}
                style={{ paddingLeft: `${level * 12 + 8}px` }}
              >
                {expandedFolders.has(node.path) ? (
                   <ChevronDown size={14} className="text-text-muted" />
                ) : (
                   <ChevronRight size={14} className="text-text-muted" />
                )}
                {expandedFolders.has(node.path) ? (
                  <FolderOpen size={16} className="text-primary/80" />
                ) : (
                  <Folder size={16} className="text-primary/60" />
                )}
                <span className="truncate">{node.name}</span>
              </button>
              
              {expandedFolders.has(node.path) && node.children && (
                <FileTree 
                  nodes={node.children} 
                  selectedFile={selectedFile} 
                  onSelect={onSelect}
                  expandedFolders={expandedFolders}
                  onToggleFolder={onToggleFolder}
                  level={level + 1}
                />
              )}
            </div>
          ) : (
            <button
              onClick={() => onSelect(node.path)}
              className={`flex items-center gap-2 w-full text-left py-1 px-2 text-sm transition-colors rounded
                ${selectedFile === node.path 
                  ? 'bg-primary/10 text-primary font-medium' 
                  : 'text-text-secondary hover:bg-dark-tertiary hover:text-text'
                }`}
              style={{ paddingLeft: `${level * 12 + 28}px` }} // Indent matching folder icon offset
            >
              <FileText size={14} className={selectedFile === node.path ? "text-primary" : "text-slate-500"} />
              <span className="truncate font-mono text-xs">{node.name}</span>
            </button>
          )}
        </div>
      ))}
    </div>
  );
}
