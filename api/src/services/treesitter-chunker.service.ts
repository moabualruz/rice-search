import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import * as path from 'path';
import * as fs from 'fs';
import { getLanguageFromExtension } from '../config/configuration';

/**
 * AST Node representing a code structure
 */
export interface AstNode {
  type: string;
  name: string;
  startLine: number;
  endLine: number;
  startColumn: number;
  endColumn: number;
  children: AstNode[];
}

/**
 * Code chunk with AST-aware boundaries
 */
export interface TreeSitterChunk {
  doc_id: string;
  path: string;
  language: string;
  symbols: string[];
  content: string;
  start_line: number;
  end_line: number;
  chunk_index: number;
  node_type?: string; // function, class, method, etc.
}

/**
 * Language-specific node types that define natural chunk boundaries
 */
const CHUNK_BOUNDARY_NODES: Record<string, string[]> = {
  // Web / Frontend
  javascript: [
    'function_declaration', 'class_declaration', 'method_definition',
    'arrow_function', 'export_statement', 'lexical_declaration',
  ],
  typescript: [
    'function_declaration', 'class_declaration', 'method_definition',
    'arrow_function', 'export_statement', 'interface_declaration',
    'type_alias_declaration', 'enum_declaration', 'module',
  ],
  tsx: [
    'function_declaration', 'class_declaration', 'method_definition',
    'arrow_function', 'export_statement', 'interface_declaration',
  ],
  jsx: [
    'function_declaration', 'class_declaration', 'method_definition',
    'arrow_function', 'export_statement',
  ],
  html: ['element', 'script_element', 'style_element'],
  css: ['rule_set', 'media_statement', 'keyframes_statement'],
  vue: ['component', 'script_element', 'template_element', 'style_element'],
  svelte: ['script', 'element', 'style_element'],

  // Systems / Backend
  python: [
    'function_definition', 'class_definition', 'decorated_definition',
    'async_function_definition', 'module',
  ],
  rust: [
    'function_item', 'impl_item', 'struct_item', 'enum_item',
    'trait_item', 'mod_item', 'macro_definition',
  ],
  go: [
    'function_declaration', 'method_declaration', 'type_declaration',
    'type_spec', 'interface_type',
  ],
  java: [
    'method_declaration', 'class_declaration', 'interface_declaration',
    'constructor_declaration', 'enum_declaration', 'annotation_type_declaration',
  ],
  kotlin: [
    'function_declaration', 'class_declaration', 'object_declaration',
    'interface_declaration', 'property_declaration',
  ],
  scala: [
    'function_definition', 'class_definition', 'object_definition',
    'trait_definition', 'val_definition',
  ],
  c: ['function_definition', 'struct_specifier', 'enum_specifier', 'type_definition'],
  cpp: [
    'function_definition', 'class_specifier', 'struct_specifier',
    'namespace_definition', 'template_declaration', 'enum_specifier',
  ],
  csharp: [
    'method_declaration', 'class_declaration', 'interface_declaration',
    'struct_declaration', 'enum_declaration', 'namespace_declaration',
  ],
  swift: [
    'function_declaration', 'class_declaration', 'protocol_declaration',
    'struct_declaration', 'enum_declaration', 'extension_declaration',
  ],

  // Scripting
  ruby: [
    'method', 'class', 'module', 'singleton_method', 'block',
  ],
  php: [
    'function_definition', 'class_declaration', 'method_declaration',
    'trait_declaration', 'interface_declaration',
  ],
  perl: ['subroutine_declaration', 'package_declaration'],
  lua: ['function_declaration', 'local_function', 'function_definition'],
  elixir: ['def', 'defp', 'defmodule', 'defmacro', 'defimpl'],
  haskell: ['function', 'data', 'class', 'instance', 'type_signature'],

  // Shell
  shell: ['function_definition', 'compound_statement'],
  bash: ['function_definition', 'compound_statement'],
  powershell: ['function_definition', 'class_statement'],

  // Data formats
  json: ['object', 'array'],
  yaml: ['block_mapping', 'block_sequence'],
  toml: ['table', 'array'],
  xml: ['element'],

  // Query
  sql: ['create_statement', 'select_statement', 'function_definition'],
  graphql: ['definition', 'type_definition', 'operation_definition'],

  // Documentation
  markdown: ['section', 'heading', 'fenced_code_block'],

  // Other
  zig: ['fn_decl', 'struct_decl', 'enum_decl'],
  dart: ['function_signature', 'class_definition', 'method_signature', 'function_body'],
  solidity: ['function_definition', 'contract_declaration', 'struct_declaration', 'event_definition', 'modifier_definition'],
  proto: ['message', 'service', 'enum'],
  terraform: ['block', 'resource', 'module'],
  nix: ['function', 'binding', 'attrset'],
  
  // Additional languages with WASM support
  elm: ['function_declaration_left', 'type_declaration', 'type_alias_declaration', 'port_annotation'],
  rescript: ['let_binding', 'type_declaration', 'module_declaration', 'external_declaration'],
  elisp: ['defun', 'defvar', 'defconst', 'defmacro', 'defcustom'],
  ocaml: ['value_definition', 'type_definition', 'module_definition', 'class_definition'],
  tlaplus: ['operator_definition', 'function_definition', 'module'],
  ql: ['predicate', 'class', 'module', 'select'],
  systemrdl: ['component_def', 'field_def', 'enum_def'],
};

/**
 * Node types that contain symbol names
 */
const SYMBOL_NODES: Record<string, string[]> = {
  python: ['function_definition', 'class_definition'],
  javascript: ['function_declaration', 'class_declaration', 'variable_declarator'],
  typescript: ['function_declaration', 'class_declaration', 'interface_declaration', 'type_alias_declaration'],
  rust: ['function_item', 'struct_item', 'enum_item', 'trait_item'],
  go: ['function_declaration', 'method_declaration', 'type_spec'],
  java: ['method_declaration', 'class_declaration', 'interface_declaration'],
  cpp: ['function_definition', 'class_specifier', 'struct_specifier'],
  c: ['function_definition', 'struct_specifier'],
};

/**
 * TreeSitterChunkerService - AST-aware code chunking using Tree-sitter
 *
 * Falls back to line-based chunking when:
 * - Language not supported
 * - Parsing fails
 * - File too large
 */
@Injectable()
export class TreeSitterChunkerService implements OnModuleInit {
  private readonly logger = new Logger(TreeSitterChunkerService.name);
  private Parser: any;
  private parsers: Map<string, any> = new Map();
  private initialized = false;

  // Chunking configuration
  private readonly MAX_CHUNK_LINES = 100;
  private readonly MIN_CHUNK_LINES = 10;
  private readonly OVERLAP_LINES = 5;
  private readonly MAX_FILE_SIZE = 500_000; // 500KB

  async onModuleInit() {
    await this.initialize();
  }

  /**
   * Initialize Tree-sitter and load language parsers
   */
  async initialize(): Promise<void> {
    if (this.initialized) return;

    try {
      // Dynamic import for web-tree-sitter
      const TreeSitter = await import('web-tree-sitter');
      await TreeSitter.default.init();
      this.Parser = TreeSitter.default;
      this.initialized = true;
      this.logger.log('Tree-sitter initialized successfully');
    } catch (error) {
      this.logger.warn(`Tree-sitter initialization failed: ${error}. Using fallback chunking.`);
      this.initialized = false;
    }
  }

  /**
   * Get or create parser for a language
   */
  private async getParser(language: string): Promise<any | null> {
    if (!this.initialized || !this.Parser) {
      return null;
    }

    if (this.parsers.has(language)) {
      return this.parsers.get(language);
    }

    // Map our language names to tree-sitter WASM files
    // Complete list of all available parsers in tree-sitter-wasms@0.1.13
    const languageMap: Record<string, string> = {
      // Web / Frontend
      javascript: 'tree-sitter-javascript',
      typescript: 'tree-sitter-typescript',
      tsx: 'tree-sitter-tsx',
      jsx: 'tree-sitter-javascript', // Use JS parser for JSX
      html: 'tree-sitter-html',
      css: 'tree-sitter-css',
      vue: 'tree-sitter-vue',
      svelte: 'tree-sitter-html', // Use HTML parser for Svelte
      
      // Systems / Backend
      python: 'tree-sitter-python',
      rust: 'tree-sitter-rust',
      go: 'tree-sitter-go',
      java: 'tree-sitter-java',
      kotlin: 'tree-sitter-kotlin',
      scala: 'tree-sitter-scala',
      c: 'tree-sitter-c',
      cpp: 'tree-sitter-cpp',
      csharp: 'tree-sitter-c_sharp',
      swift: 'tree-sitter-swift',
      objectivec: 'tree-sitter-objc',
      dart: 'tree-sitter-dart',
      
      // Scripting
      ruby: 'tree-sitter-ruby',
      php: 'tree-sitter-php',
      lua: 'tree-sitter-lua',
      elixir: 'tree-sitter-elixir',
      ocaml: 'tree-sitter-ocaml',
      elm: 'tree-sitter-elm',
      rescript: 'tree-sitter-rescript',
      elisp: 'tree-sitter-elisp',
      
      // Shell / Config
      shell: 'tree-sitter-bash',
      bash: 'tree-sitter-bash',
      zsh: 'tree-sitter-bash', // Use bash parser for zsh
      
      // Data / Config formats
      json: 'tree-sitter-json',
      jsonc: 'tree-sitter-json',
      json5: 'tree-sitter-json',
      yaml: 'tree-sitter-yaml',
      toml: 'tree-sitter-toml',
      
      // Query / Database
      ql: 'tree-sitter-ql',
      codeql: 'tree-sitter-ql',
      
      // Embedded templates (EJS, ERB, etc.)
      ejs: 'tree-sitter-embedded_template',
      erb: 'tree-sitter-embedded_template',
      
      // Other languages with WASM support
      zig: 'tree-sitter-zig',
      solidity: 'tree-sitter-solidity',
      tlaplus: 'tree-sitter-tlaplus',
      systemrdl: 'tree-sitter-systemrdl',
    };

    const wasmName = languageMap[language];
    if (!wasmName) {
      return null;
    }

    try {
      const parser = new this.Parser();
      
      // Resolve WASM path - check multiple locations
      // Priority: Docker wasm dir > local wasm dir > node_modules
      const possiblePaths = [
        // Docker wasm directory (downloaded during build)
        path.join('/app', 'wasm', `${wasmName}.wasm`),
        // Local wasm directory (repo version)
        path.join(process.cwd(), 'wasm', `${wasmName}.wasm`),
        path.join(__dirname, '..', '..', 'wasm', `${wasmName}.wasm`),
        // tree-sitter-wasms npm package (fallback)
        path.join(process.cwd(), 'node_modules', 'tree-sitter-wasms', 'out', `${wasmName}.wasm`),
        path.join(__dirname, '..', '..', 'node_modules', 'tree-sitter-wasms', 'out', `${wasmName}.wasm`),
        path.join('/app', 'node_modules', 'tree-sitter-wasms', 'out', `${wasmName}.wasm`),
      ];

      let wasmPath: string | null = null;
      for (const p of possiblePaths) {
        if (fs.existsSync(p)) {
          wasmPath = p;
          break;
        }
      }

      if (!wasmPath) {
        this.logger.debug(`WASM file not found for ${language}: ${wasmName}.wasm`);
        return null;
      }

      const Lang = await this.Parser.Language.load(wasmPath);
      parser.setLanguage(Lang);
      this.parsers.set(language, parser);
      this.logger.debug(`Loaded parser for ${language} from ${wasmPath}`);
      return parser;
    } catch (error) {
      this.logger.debug(`Failed to load parser for ${language}: ${error}`);
      return null;
    }
  }

  /**
   * Extract symbol name from an AST node
   */
  private extractSymbolName(node: any, language: string): string | null {
    try {
      // Different languages have different patterns for symbol names
      const nameChild = node.childForFieldName?.('name');
      if (nameChild) {
        return nameChild.text;
      }

      // Fallback: look for identifier child
      for (let i = 0; i < node.childCount; i++) {
        const child = node.child(i);
        if (child.type === 'identifier' || child.type === 'type_identifier') {
          return child.text;
        }
      }
    } catch {
      // Ignore extraction errors
    }
    return null;
  }

  /**
   * Find all chunk boundary nodes in the AST
   */
  private findChunkBoundaries(
    rootNode: any,
    language: string,
    sourceLines: string[],
  ): AstNode[] {
    const boundaryTypes = CHUNK_BOUNDARY_NODES[language] || [];
    const symbolTypes = SYMBOL_NODES[language] || [];
    const boundaries: AstNode[] = [];

    const visit = (node: any) => {
      if (boundaryTypes.includes(node.type)) {
        const name = this.extractSymbolName(node, language) || '';
        boundaries.push({
          type: node.type,
          name,
          startLine: node.startPosition.row + 1,
          endLine: node.endPosition.row + 1,
          startColumn: node.startPosition.column,
          endColumn: node.endPosition.column,
          children: [],
        });
      }

      // Recurse into children
      for (let i = 0; i < node.childCount; i++) {
        visit(node.child(i));
      }
    };

    visit(rootNode);
    return boundaries;
  }

  /**
   * Extract all symbol names from code using regex (fallback)
   */
  private extractSymbolsRegex(content: string, language: string): string[] {
    const symbols: Set<string> = new Set();

    const patterns: Record<string, RegExp[]> = {
      python: [
        /def\s+(\w+)\s*\(/g,
        /class\s+(\w+)\s*[:(]/g,
      ],
      javascript: [
        /function\s+(\w+)\s*\(/g,
        /const\s+(\w+)\s*=/g,
        /class\s+(\w+)\s*[{extends]/g,
        /(\w+)\s*:\s*function/g,
      ],
      typescript: [
        /function\s+(\w+)\s*[<(]/g,
        /const\s+(\w+)\s*[=:]/g,
        /class\s+(\w+)\s*[{<extends]/g,
        /interface\s+(\w+)\s*[{<]/g,
        /type\s+(\w+)\s*[=<]/g,
      ],
      rust: [
        /fn\s+(\w+)\s*[<(]/g,
        /struct\s+(\w+)\s*[{<]/g,
        /enum\s+(\w+)\s*[{<]/g,
        /trait\s+(\w+)\s*[{<]/g,
        /impl\s+(\w+)/g,
      ],
      go: [
        /func\s+(\w+)\s*\(/g,
        /func\s*\(\w+\s+\*?\w+\)\s+(\w+)\s*\(/g,
        /type\s+(\w+)\s+struct/g,
      ],
      java: [
        /class\s+(\w+)\s*[{<extends]/g,
        /interface\s+(\w+)\s*[{<]/g,
        /(?:public|private|protected)?\s*(?:static)?\s*\w+\s+(\w+)\s*\(/g,
      ],
      cpp: [
        /class\s+(\w+)\s*[{:]/g,
        /struct\s+(\w+)\s*[{:]/g,
        /(\w+)\s*\([^)]*\)\s*(?:const)?\s*[{;]/g,
      ],
      c: [
        /struct\s+(\w+)\s*[{;]/g,
        /(\w+)\s*\([^)]*\)\s*[{;]/g,
      ],
    };

    // Use JavaScript patterns as fallback for unknown languages
    const langPatterns = patterns[language] || patterns.javascript || [];
    
    // Common keywords to filter out
    const keywords = new Set([
      'if', 'else', 'for', 'while', 'return', 'import', 'from',
      'export', 'default', 'const', 'let', 'var', 'function',
      'class', 'interface', 'type', 'struct', 'enum', 'void',
      'int', 'string', 'bool', 'true', 'false', 'null', 'undefined',
    ]);

    for (const pattern of langPatterns) {
      let match;
      const regex = new RegExp(pattern.source, pattern.flags);
      while ((match = regex.exec(content)) !== null) {
        const name = match[1];
        if (name && name.length > 1 && name.length < 50 && !keywords.has(name.toLowerCase())) {
          symbols.add(name);
        }
      }
    }

    return Array.from(symbols);
  }

  /**
   * Chunk code using Tree-sitter AST
   * Always succeeds - falls back to line-based chunking if AST parsing unavailable
   */
  async chunkWithTreeSitter(
    filePath: string,
    content: string,
  ): Promise<TreeSitterChunk[]> {
    const ext = path.extname(filePath);
    let language = getLanguageFromExtension(ext);
    
    // Use 'text' for unknown languages - still index them!
    if (language === 'unknown') {
      language = 'text';
    }
    
    const lines = content.split('\n');

    // Skip AST parsing for large files - use line-based
    if (content.length > this.MAX_FILE_SIZE) {
      this.logger.debug(`File too large for AST parsing, using line-based: ${filePath}`);
      return this.chunkByLines(filePath, content, language);
    }

    // Try to get parser - if unavailable, fall back gracefully
    let parser: any = null;
    try {
      parser = await this.getParser(language);
    } catch (error) {
      // Parser loading failed - not a problem, use fallback
      this.logger.debug(`Parser unavailable for ${language}, using line-based: ${filePath}`);
    }
    
    if (!parser) {
      // No parser available - fall back to line-based (this is fine!)
      return this.chunkByLines(filePath, content, language);
    }

    try {
      const tree = parser.parse(content);
      const boundaries = this.findChunkBoundaries(tree.rootNode, language, lines);

      if (boundaries.length === 0) {
        return this.chunkByLines(filePath, content, language);
      }

      // Create chunks from boundaries
      const chunks: TreeSitterChunk[] = [];
      let chunkIndex = 0;

      for (const boundary of boundaries) {
        const startLine = boundary.startLine;
        const endLine = boundary.endLine;
        const chunkLines = lines.slice(startLine - 1, endLine);
        const chunkContent = chunkLines.join('\n');

        // Skip very small chunks
        if (chunkLines.length < this.MIN_CHUNK_LINES && chunks.length > 0) {
          // Merge with previous chunk if possible
          const prevChunk = chunks[chunks.length - 1];
          if (prevChunk.end_line === startLine - 1) {
            prevChunk.content += '\n' + chunkContent;
            prevChunk.end_line = endLine;
            if (boundary.name) {
              prevChunk.symbols.push(boundary.name);
            }
            continue;
          }
        }

        // Extract symbols from this chunk
        const symbols = this.extractSymbolsRegex(chunkContent, language);
        if (boundary.name && !symbols.includes(boundary.name)) {
          symbols.unshift(boundary.name);
        }

        chunks.push({
          doc_id: this.generateDocId(filePath, chunkIndex, chunkContent),
          path: filePath,
          language,
          symbols,
          content: chunkContent,
          start_line: startLine,
          end_line: endLine,
          chunk_index: chunkIndex,
          node_type: boundary.type,
        });

        chunkIndex++;
      }

      // If we have chunks, return them
      if (chunks.length > 0) {
        this.logger.debug(`AST chunking: ${filePath} -> ${chunks.length} chunks`);
        return chunks;
      }
    } catch (error) {
      this.logger.debug(`AST parsing failed for ${filePath}: ${error}`);
    }

    // Fallback to line-based chunking
    return this.chunkByLines(filePath, content, language);
  }

  /**
   * Fallback: chunk by lines with overlap
   */
  chunkByLines(
    filePath: string,
    content: string,
    language: string,
  ): TreeSitterChunk[] {
    const lines = content.split('\n');
    const chunks: TreeSitterChunk[] = [];

    // Small file - single chunk
    if (lines.length <= this.MAX_CHUNK_LINES) {
      const symbols = this.extractSymbolsRegex(content, language);
      return [{
        doc_id: this.generateDocId(filePath, 0, content),
        path: filePath,
        language,
        symbols,
        content,
        start_line: 1,
        end_line: lines.length,
        chunk_index: 0,
      }];
    }

    // Chunk larger files
    let chunkIndex = 0;
    let startLine = 0;

    while (startLine < lines.length) {
      const endLine = Math.min(startLine + this.MAX_CHUNK_LINES, lines.length);
      const chunkLines = lines.slice(startLine, endLine);
      const chunkContent = chunkLines.join('\n');

      const symbols = this.extractSymbolsRegex(chunkContent, language);

      chunks.push({
        doc_id: this.generateDocId(filePath, chunkIndex, chunkContent),
        path: filePath,
        language,
        symbols,
        content: chunkContent,
        start_line: startLine + 1,
        end_line: endLine,
        chunk_index: chunkIndex,
      });

      chunkIndex++;
      startLine = endLine - this.OVERLAP_LINES;

      if (startLine >= lines.length - this.OVERLAP_LINES) {
        break;
      }
    }

    return chunks;
  }

  /**
   * Generate stable document ID
   */
  private generateDocId(filePath: string, chunkIndex: number, content: string): string {
    const hashInput = `${filePath}:${chunkIndex}:${content.length}`;
    let hash = 0;
    for (let i = 0; i < hashInput.length; i++) {
      const char = hashInput.charCodeAt(i);
      hash = (hash << 5) - hash + char;
      hash = hash & hash;
    }
    return `${filePath}#${chunkIndex}#${Math.abs(hash).toString(16)}`;
  }

  /**
   * Check if content is likely binary
   */
  isBinary(content: string): boolean {
    if (content.includes('\0')) {
      return true;
    }

    let nonPrintable = 0;
    const sampleSize = Math.min(content.length, 8000);

    for (let i = 0; i < sampleSize; i++) {
      const code = content.charCodeAt(i);
      if (code < 32 && code !== 9 && code !== 10 && code !== 13) {
        nonPrintable++;
      }
    }

    return nonPrintable / sampleSize > 0.1;
  }
}
