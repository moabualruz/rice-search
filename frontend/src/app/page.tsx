"use client";

import { useState, memo } from "react";
import Image from "next/image";
import { Button, Input, Card } from "@/components/ui-elements";
import {
  Search,
  Sparkles,
  Database,
  FileText,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Copy,
  Check,
  Loader2,
  Minimize2,
  Maximize2,
} from "lucide-react";
import ReactMarkdown from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism";
import { api, type SearchResult } from "@/lib/api";

// Helper to get file extension for syntax highlighting
function getLanguage(filepath?: string): string {
  if (!filepath) return "text";
  const ext = filepath.split(".").pop()?.toLowerCase() || "";
  const map: Record<string, string> = {
    py: "python",
    js: "javascript",
    ts: "typescript",
    tsx: "tsx",
    jsx: "jsx",
    rs: "rust",
    go: "go",
    java: "java",
    cpp: "cpp",
    c: "c",
    rb: "ruby",
    md: "markdown",
    json: "json",
    yaml: "yaml",
    yml: "yaml",
    toml: "toml",
    html: "html",
    css: "css",
    sh: "bash",
    sql: "sql",
    dockerfile: "docker",
  };
  return map[ext] || "text";
}

// Helper to format relevance score
function formatRelevance(score: number): { label: string; color: string } {
  // Cross-encoder (ms-marco-MiniLM-L-12-v2) returns scores typically in range [-10, +10]
  // Positive scores = relevant, negative = less relevant
  // We normalize to 0-100% using sigmoid-like mapping

  // Map [-10, +10] to [0, 1] using sigmoid function
  // This gives smooth 0-100% range with 50% at score=0
  const normalized = 1 / (1 + Math.exp(-score));
  const pct = Math.round(normalized * 100);

  // Thresholds based on normalized scores
  if (normalized >= 0.75)  // score > ~1.1
    return { label: `High (${pct}%)`, color: "text-green-400 bg-green-500/20" };
  if (normalized >= 0.60)  // score > ~0.4
    return {
      label: `Good (${pct}%)`,
      color: "text-yellow-400 bg-yellow-500/20",
    };
  if (normalized >= 0.40)  // score > -0.4
    return {
      label: `Fair (${pct}%)`,
      color: "text-orange-400 bg-orange-500/20",
    };
  return { label: `Low (${pct}%)`, color: "text-slate-400 bg-slate-500/20" };
}

// Expandable result card component
const ResultCard = memo(function ResultCard({ hit, index }: { hit: SearchResult; index: number }) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);
  const [rawMarkdown, setRawMarkdown] = useState(false);
  const [fullContent, setFullContent] = useState<string | null>(null);
  const [loadingFull, setLoadingFull] = useState(false);

  const filePath =
    hit.file_path ||
    hit.metadata?.file_path ||
    hit.metadata?.client_system_path ||
    "Unknown Document";
  const score = hit.rerank_score ?? hit.score ?? 0;
  const relevance = formatRelevance(score);
  const language = getLanguage(filePath);
  const isPDF = filePath.toLowerCase().endsWith(".pdf");
  const isMarkdown = filePath.toLowerCase().endsWith(".md");

  const handleCopy = async () => {
    await navigator.clipboard.writeText(hit.text || "");
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleViewFullFile = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (fullContent) {
      setFullContent(null);
      return;
    }

    try {
      setLoadingFull(true);
      const data = await api.getFileContent(filePath);
      setFullContent(data.content);
    } catch (err) {
      console.error("Failed to load file:", err);
    } finally {
      setLoadingFull(false);
    }
  };

  return (
    <Card
      className="hover:border-slate-700 transition-colors group cursor-pointer"
      onClick={() => setExpanded(!expanded)}
    >
      <div className="flex items-start gap-4">
        <div className="mt-1 p-2 bg-slate-800 rounded-lg text-indigo-400 group-hover:text-indigo-300">
          <FileText size={20} />
        </div>
        <div className="flex-1 space-y-2 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <h3
              className="font-medium text-slate-200 text-sm truncate flex-1"
              title={filePath}
            >
              {filePath.split("/").pop() || filePath}
            </h3>
            <div className="flex items-center gap-2 shrink-0">
              <span
                className={`text-xs px-2 py-1 rounded-full ${relevance.color}`}
              >
                {relevance.label}
              </span>
              {expanded ? (
                <ChevronUp size={16} className="text-slate-500" />
              ) : (
                <ChevronDown size={16} className="text-slate-500" />
              )}
            </div>
          </div>

          {/* Preview snippet */}
          {!expanded && (
            <p className="text-slate-400 text-sm line-clamp-2">{hit.text}</p>
          )}

          {/* Expanded content */}
          {expanded && (
            <div
              className="mt-4 space-y-3"
              onClick={(e) => e.stopPropagation()}
            >
              {/* File path breadcrumb */}
              <div className="text-xs text-slate-500 font-mono overflow-x-auto">
                {filePath}
                {hit.start_line && hit.end_line && (
                  <span className="ml-2 text-indigo-400">
                    L{hit.start_line}-{hit.end_line}
                  </span>
                )}
              </div>

              {/* Actions */}
              <div className="flex gap-2">
                <button
                  onClick={handleCopy}
                  className="flex items-center gap-1 text-xs px-2 py-1 bg-slate-800 hover:bg-slate-700 rounded text-slate-300"
                >
                  {copied ? <Check size={12} /> : <Copy size={12} />}
                  {copied ? "Copied!" : "Copy"}
                </button>
                {isMarkdown && (
                  <button
                    onClick={() => setRawMarkdown(!rawMarkdown)}
                    className={`flex items-center gap-1 text-xs px-2 py-1 rounded ${
                      rawMarkdown
                        ? "bg-indigo-600 text-white"
                        : "bg-slate-800 hover:bg-slate-700 text-slate-300"
                    }`}
                  >
                    {rawMarkdown ? "Raw" : "Rendered"}
                  </button>
                )}
                {isPDF && (
                  <a
                    href={`/api/files/view?path=${encodeURIComponent(
                      filePath
                    )}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-1 text-xs px-2 py-1 bg-slate-800 hover:bg-slate-700 rounded text-slate-300"
                  >
                    <ExternalLink size={12} /> Open PDF
                  </a>
                )}
                
                <button
                  onClick={handleViewFullFile}
                  disabled={loadingFull}
                  className={`flex items-center gap-1 text-xs px-2 py-1 rounded ${
                    fullContent
                      ? "bg-indigo-600 text-white"
                      : "bg-slate-800 hover:bg-slate-700 text-slate-300"
                  }`}
                >
                  {loadingFull ? (
                     <Loader2 size={12} className="animate-spin" />
                  ) : fullContent ? (
                    <Minimize2 size={12} />
                  ) : (
                    <Maximize2 size={12} />
                  )}
                  {fullContent ? "Close Full File" : "View Full File"}
                </button>
              </div>

              {/* Content preview or Full File view */}
              <div className="rounded-lg overflow-hidden border border-slate-700">
                  {fullContent ? (
                  (() => {
                    // Safety check for large files
                    const MAX_LINES = 2000;
                    // Simple line count estimate (or split)
                    const isLarge = fullContent.length > 50000 && (fullContent.match(/\n/g)||[]).length > MAX_LINES;
                    let displayContent = fullContent;
                    
                    if (isLarge) {
                       // Truncate to avoid freezing
                       const lines = fullContent.split('\n');
                       if (lines.length > MAX_LINES) {
                           displayContent = lines.slice(0, MAX_LINES).join('\n');
                       }
                    }

                    return (
                      <div className="relative">
                        {isLarge && (
                          <div className="bg-yellow-500/10 border-l-4 border-yellow-500 p-2 mb-2 text-yellow-200 text-xs">
                            Warning: File is very large. Showing first {MAX_LINES} lines to prevent browser freeze. 
                            {!rawMarkdown && " Switch to Raw/Copy to retrieve full content."}
                          </div>
                        )}
                        {isMarkdown && !rawMarkdown ? (
                            <div className="prose prose-invert prose-sm max-w-none p-4 bg-slate-900">
                              <ReactMarkdown>{displayContent}</ReactMarkdown>
                            </div>
                          ) : (
                           <SyntaxHighlighter
                             language={isMarkdown && rawMarkdown ? "markdown" : language}
                             style={oneDark}
                             showLineNumbers={true}
                             wrapLines={false} // wrapLines=true is massive perf hit
                             lineProps={(lineNumber): any => {
                               const start = hit.start_line || 0;
                               const end = hit.end_line || 0;
                               if (lineNumber >= start && lineNumber <= end) {
                                 return { style: { display: "block", backgroundColor: "rgba(99, 102, 241, 0.2)" } };
                               }
                               return {};
                             }}
                             customStyle={{
                               margin: 0,
                               padding: "1rem",
                               fontSize: "0.75rem",
                               maxHeight: "600px", 
                               overflow: "auto",
                             }}
                           >
                             {displayContent}
                           </SyntaxHighlighter>
                          )}
                      </div>
                    );
                  })()
                ) : isMarkdown ? (
                  rawMarkdown ? (
                    <SyntaxHighlighter
                      language="markdown"
                      style={oneDark}
                      showLineNumbers={true}
                      customStyle={{
                        margin: 0,
                        padding: "1rem",
                        fontSize: "0.75rem",
                        maxHeight: "400px",
                        overflow: "auto",
                      }}
                    >
                      {hit.text || ""}
                    </SyntaxHighlighter>
                  ) : (
                    <div className="prose prose-invert prose-sm max-w-none p-4 bg-slate-900">
                      <ReactMarkdown>{hit.text}</ReactMarkdown>
                    </div>
                  )
                ) : (
                  <SyntaxHighlighter
                    language={language}
                    style={oneDark}
                    customStyle={{
                      margin: 0,
                      padding: "1rem",
                      fontSize: "0.75rem",
                      maxHeight: "400px",
                      overflow: "auto",
                    }}
                    showLineNumbers={hit.start_line !== undefined}
                    startingLineNumber={hit.start_line || 1}
                  >
                    {hit.text || ""}
                  </SyntaxHighlighter>
                )}
              </div>

              {/* Metadata badges */}
              <div className="flex flex-wrap gap-2 mt-2">
                {hit.metadata?.chunk_type && (
                  <span className="text-xs px-2 py-0.5 bg-indigo-500/20 text-indigo-300 rounded">
                    {hit.metadata.chunk_type}
                  </span>
                )}
                {hit.metadata?.language && (
                  <span className="text-xs px-2 py-0.5 bg-purple-500/20 text-purple-300 rounded">
                    {hit.metadata.language}
                  </span>
                )}
                {hit.metadata?.symbols?.length > 0 && (
                  <span className="text-xs px-2 py-0.5 bg-green-500/20 text-green-300 rounded font-mono">
                    {hit.metadata.symbols.join(", ")}
                  </span>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </Card>
  );
});

export default function Home() {
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState<"search" | "rag">("rag");
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [answer, setAnswer] = useState<string | null>(null);
  const [stepsTaken, setStepsTaken] = useState<number>(0);
  const [searchTime, setSearchTime] = useState<number>(0);

  const handleSearch = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setAnswer(null);
    setResults([]);
    setStepsTaken(0);
    const startTime = Date.now();

    try {
      const res = await api.search(query, mode);
      setSearchTime((Date.now() - startTime) / 1000);

      if (mode === "rag") {
        setAnswer(res.answer || "No answer generated.");
        setResults(res.sources || []);
        // @ts-ignore - steps_taken is new
        if (res.steps_taken) setStepsTaken(res.steps_taken);
      } else {
        setResults(res.results || []);
      }
    } catch (err) {
      console.error(err);
      setAnswer("Error occurred while searching. Please try again.");
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
              alt="rice ?earch"
              width={64}
              height={64}
              className="rounded-xl"
            />
          </div>
          <h1 className="text-5xl font-extrabold tracking-tight sm:text-7xl text-primary">
            rice ?earch
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
                {mode === "rag" ? <Sparkles size={20} /> : <Search size={20} />}
              </div>
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={
                  mode === "rag" ? "Ask anything..." : "Search documents..."
                }
                className="border-0 bg-transparent focus:ring-0 text-lg h-12"
              />
              <div className="flex bg-slate-800 rounded-lg p-1 gap-1">
                <button
                  type="button"
                  onClick={() => setMode("search")}
                  className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                    mode === "search"
                      ? "bg-indigo-500 text-white"
                      : "text-slate-400 hover:text-white"
                  }`}
                >
                  Search
                </button>
                <button
                  type="button"
                  onClick={() => setMode("rag")}
                  className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                    mode === "rag"
                      ? "bg-purple-500 text-white"
                      : "text-slate-400 hover:text-white"
                  }`}
                >
                  Ask AI
                </button>
              </div>
              <Button
                type="submit"
                size="lg"
                disabled={loading}
                className="rounded-lg h-12 w-12 p-0 flex items-center justify-center"
              >
                {loading ? (
                  <Loader2 size={20} className="animate-spin" />
                ) : (
                  <ChevronRight size={24} />
                )}
              </Button>
            </div>
          </div>
        </form>

        {/* Loading indicator for Ask AI */}
        {loading && mode === "rag" && (
          <div className="flex items-center justify-center gap-3 text-purple-400">
            <Loader2 size={20} className="animate-spin" />
            <span className="text-sm">
              Thinking and searching across your documents...
            </span>
          </div>
        )}

        {/* Results */}
        <div className="w-full max-w-3xl mx-auto text-left space-y-6 mt-12 pb-20">
          {/* Search stats */}
          {!loading && results.length > 0 && (
            <div className="text-xs text-slate-500 px-1">
              Found {results.length} results in {searchTime.toFixed(2)}s
            </div>
          )}

          {/* AI Answer */}
          {answer && (
            <Card className="border-purple-500/20 bg-purple-500/5">
              <div className="flex items-center gap-2 mb-4 text-purple-400 justify-between">
                <div className="flex items-center gap-2">
                  <Sparkles size={18} />
                  <span className="text-sm font-semibold uppercase tracking-wider">
                    AI Answer
                  </span>
                </div>
                {stepsTaken > 1 && (
                  <span className="text-xs bg-purple-500/20 px-2 py-0.5 rounded text-purple-300">
                    Deep Dive: {stepsTaken} steps
                  </span>
                )}
              </div>
              <div className="prose prose-invert max-w-none text-slate-200">
                <ReactMarkdown
                  components={{
                    code({ node, inline, className, children, ...props }: any) {
                      const match = /language-(\w+)/.exec(className || "");
                      return !inline && match ? (
                        <SyntaxHighlighter
                          style={oneDark as any}
                          language={match[1]}
                          PreTag="div"
                          {...props}
                        >
                          {String(children).replace(/\n$/, "")}
                        </SyntaxHighlighter>
                      ) : (
                        <code className={className} {...props}>
                          {children}
                        </code>
                      );
                    },
                  }}
                >
                  {answer}
                </ReactMarkdown>
              </div>
            </Card>
          )}

          {/* Sources / Hits */}
          {results.length > 0 && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-slate-500 px-1">
                <Database size={16} />
                <span className="text-xs font-medium uppercase tracking-wider">
                  {mode === "rag" ? "Sources" : "Results"}
                </span>
              </div>
              {results.map((hit, i) => (
                <ResultCard key={i} hit={hit} index={i} />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Footer Links */}
      <div className="fixed bottom-4 right-4 flex gap-4">
        <a
          href="/stores"
          className="text-xs text-slate-600 hover:text-slate-400 transition-colors"
        >
          Manage Stores
        </a>
        <a
          href="/browse"
          className="text-xs text-slate-600 hover:text-slate-400 transition-colors"
        >
          Browse Files
        </a>
        <a
          href="/admin"
          className="text-xs text-slate-600 hover:text-slate-400 transition-colors"
        >
          Admin Dashboard &rarr;
        </a>
      </div>
    </main>
  );
}
