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
    useCargo: process.env.TANTIVY_USE_CARGO === 'true',
    projectDir: process.env.TANTIVY_PROJECT_DIR || '',
  },

  // Redis (for BullMQ job queue)
  redis: {
    host: process.env.REDIS_HOST || 'localhost',
    port: parseInt(process.env.REDIS_PORT || '6379', 10),
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

// Language file extensions - comprehensive list
export const LANGUAGE_EXTENSIONS: Record<string, string[]> = {
  // Web / Frontend
  javascript: ['.js', '.mjs', '.cjs'],
  typescript: ['.ts', '.mts', '.cts'],
  tsx: ['.tsx'],
  jsx: ['.jsx'],
  html: ['.html', '.htm', '.xhtml'],
  css: ['.css'],
  scss: ['.scss', '.sass', '.less'],
  vue: ['.vue'],
  svelte: ['.svelte'],
  
  // Systems / Backend
  python: ['.py', '.pyi', '.pyw', '.pyx'],
  rust: ['.rs'],
  go: ['.go'],
  java: ['.java'],
  kotlin: ['.kt', '.kts'],
  scala: ['.scala', '.sc'],
  c: ['.c', '.h'],
  cpp: ['.cpp', '.cc', '.cxx', '.hpp', '.hh', '.hxx', '.c++', '.h++'],
  csharp: ['.cs', '.csx'],
  swift: ['.swift'],
  objectivec: ['.m', '.mm'],
  
  // Scripting
  ruby: ['.rb', '.rake', '.gemspec', '.ru'],
  php: ['.php', '.phtml', '.php4', '.php5'],
  perl: ['.pl', '.pm', '.pod', '.t'],
  lua: ['.lua'],
  r: ['.r', '.R', '.Rmd'],
  julia: ['.jl'],
  elixir: ['.ex', '.exs'],
  erlang: ['.erl', '.hrl'],
  haskell: ['.hs', '.lhs'],
  ocaml: ['.ml', '.mli'],
  clojure: ['.clj', '.cljs', '.cljc', '.edn'],
  lisp: ['.lisp', '.lsp', '.cl'],
  scheme: ['.scm', '.ss'],
  racket: ['.rkt'],
  elm: ['.elm'],
  rescript: ['.res', '.resi'],
  elisp: ['.el', '.elc'],
  
  // Shell / Config
  shell: ['.sh', '.bash', '.zsh'],
  fish: ['.fish'],
  powershell: ['.ps1', '.psm1', '.psd1'],
  dockerfile: ['Dockerfile', '.dockerfile'],
  makefile: ['Makefile', 'makefile', '.mk'],
  cmake: ['CMakeLists.txt', '.cmake'],
  
  // Data / Config formats
  json: ['.json', '.jsonc', '.json5'],
  yaml: ['.yaml', '.yml'],
  toml: ['.toml'],
  xml: ['.xml', '.xsl', '.xslt', '.xsd', '.svg'],
  ini: ['.ini', '.cfg', '.conf'],
  
  // Query / Database
  sql: ['.sql', '.mysql', '.pgsql'],
  graphql: ['.graphql', '.gql'],
  ql: ['.ql', '.qll'],
  codeql: ['.ql', '.qll'],
  
  // Documentation
  markdown: ['.md', '.mdx', '.markdown'],
  rst: ['.rst'],
  latex: ['.tex', '.sty', '.cls'],
  
  // Other languages
  zig: ['.zig'],
  nim: ['.nim', '.nims'],
  d: ['.d'],
  dart: ['.dart'],
  groovy: ['.groovy', '.gradle'],
  verilog: ['.v', '.sv', '.svh'],
  vhdl: ['.vhd', '.vhdl'],
  wgsl: ['.wgsl'],
  glsl: ['.glsl', '.vert', '.frag', '.geom'],
  hlsl: ['.hlsl', '.fx'],
  cuda: ['.cu', '.cuh'],
  proto: ['.proto'],
  protobuf: ['.proto'],
  thrift: ['.thrift'],
  hcl: ['.hcl'],
  terraform: ['.tf', '.tfvars'],
  nix: ['.nix'],
  solidity: ['.sol'],
  
  // Embedded templates
  ejs: ['.ejs'],
  erb: ['.erb', '.rhtml'],
  
  // Specialized languages with WASM support
  tlaplus: ['.tla'],
  systemrdl: ['.rdl'],
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
