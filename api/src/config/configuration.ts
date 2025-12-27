export default () => ({
  // Server
  port: parseInt(process.env.PORT || '8080', 10),
  nodeEnv: process.env.NODE_ENV || 'development',

  // Milvus connection
  milvus: {
    host: process.env.MILVUS_HOST || 'localhost',
    port: parseInt(process.env.MILVUS_PORT || '19530', 10),
  },

  // Embeddings service (TEI)
  embeddings: {
    url: process.env.EMBEDDINGS_URL || 'http://localhost:8081',
    model: process.env.EMBEDDING_MODEL || 'BAAI/bge-base-en-v1.5',
    dim: parseInt(process.env.EMBEDDING_DIM || '768', 10),
  },

  // Data directories
  data: {
    dir: process.env.DATA_DIR || '/data',
    tantivyDir: process.env.TANTIVY_INDEX_DIR || '/tantivy',
  },

  // Search configuration
  search: {
    sparseTopK: parseInt(process.env.SPARSE_TOPK || '200', 10),
    denseTopK: parseInt(process.env.DENSE_TOPK || '80', 10),
    finalTopK: parseInt(process.env.FINAL_TOPK || '20', 10),
    rrfK: parseInt(process.env.RRF_K || '60', 10),
  },

  // File indexing limits
  indexing: {
    maxFileSizeMb: parseInt(process.env.MAX_FILE_SIZE_MB || '10', 10),
    maxFilesPerBatch: parseInt(process.env.MAX_FILES_PER_BATCH || '100', 10),
  },

  // Authentication
  auth: {
    mode: process.env.AUTH_MODE || 'none', // none, api_key
    apiKey: process.env.API_KEY || '',
  },

  // Tantivy CLI
  tantivy: {
    cliPath: process.env.TANTIVY_CLI_PATH || '/usr/local/bin/tantivy-cli',
  },
});

// Default ignore patterns
export const DEFAULT_IGNORE_PATTERNS = [
  // Version control
  '.git',
  '.svn',
  '.hg',

  // Dependencies
  'node_modules',
  'vendor',
  '.venv',
  'venv',
  '__pycache__',
  '.tox',
  '.nox',

  // Build outputs
  'dist',
  'build',
  'target',
  '.next',
  'out',
  '_build',

  // IDE/Editor
  '.vscode',
  '.idea',
  '*.swp',
  '*.swo',

  // Caches
  '.cache',
  '.pytest_cache',
  '.mypy_cache',
  '.ruff_cache',

  // Logs
  '*.log',
  'logs',

  // OS files
  '.DS_Store',
  'Thumbs.db',

  // Binaries and media
  '*.pyc',
  '*.pyo',
  '*.so',
  '*.dylib',
  '*.dll',
  '*.exe',
  '*.bin',
  '*.o',
  '*.a',
  '*.class',
  '*.jar',
  '*.war',
  '*.png',
  '*.jpg',
  '*.jpeg',
  '*.gif',
  '*.ico',
  '*.svg',
  '*.woff',
  '*.woff2',
  '*.ttf',
  '*.eot',
  '*.mp3',
  '*.mp4',
  '*.avi',
  '*.mov',
  '*.pdf',
  '*.zip',
  '*.tar',
  '*.gz',
  '*.rar',
  '*.7z',
];

// Language file extensions
export const LANGUAGE_EXTENSIONS: Record<string, string[]> = {
  python: ['.py', '.pyi', '.pyw'],
  javascript: ['.js', '.mjs', '.cjs'],
  typescript: ['.ts', '.mts', '.cts'],
  tsx: ['.tsx'],
  jsx: ['.jsx'],
  rust: ['.rs'],
  go: ['.go'],
  java: ['.java'],
  c: ['.c', '.h'],
  cpp: ['.cpp', '.cc', '.cxx', '.hpp', '.hh', '.hxx'],
  csharp: ['.cs'],
  ruby: ['.rb'],
  php: ['.php'],
  swift: ['.swift'],
  kotlin: ['.kt', '.kts'],
  scala: ['.scala'],
  shell: ['.sh', '.bash', '.zsh'],
  yaml: ['.yaml', '.yml'],
  json: ['.json'],
  toml: ['.toml'],
  markdown: ['.md', '.mdx'],
  html: ['.html', '.htm'],
  css: ['.css'],
  scss: ['.scss', '.sass'],
  sql: ['.sql'],
  graphql: ['.graphql', '.gql'],
};

export function getLanguageFromExtension(ext: string): string {
  const lowerExt = ext.toLowerCase();
  for (const [lang, extensions] of Object.entries(LANGUAGE_EXTENSIONS)) {
    if (extensions.includes(lowerExt)) {
      return lang;
    }
  }
  return 'unknown';
}
