import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import * as path from 'path';
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
  python: [
    'function_definition',
    'class_definition',
    'decorated_definition',
    'module',
  ],
  javascript: [
    'function_declaration',
    'class_declaration',
    'method_definition',
    'arrow_function',
    'export_statement',
  ],
  typescript: [
    'function_declaration',
    'class_declaration',
    'method_definition',
    'arrow_function',
    'export_statement',
    'interface_declaration',
    'type_alias_declaration',
  ],
  rust: [
    'function_item',
    'impl_item',
    'struct_item',
    'enum_item',
    'trait_item',
    'mod_item',
  ],
  go: [
    'function_declaration',
    'method_declaration',
    'type_declaration',
  ],
  java: [
    'method_declaration',
    'class_declaration',
    'interface_declaration',
    'constructor_declaration',
  ],
  cpp: [
    'function_definition',
    'class_specifier',
    'struct_specifier',
    'namespace_definition',
  ],
  c: [
    'function_definition',
    'struct_specifier',
  ],
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
    const languageMap: Record<string, string> = {
      python: 'tree-sitter-python',
      javascript: 'tree-sitter-javascript',
      typescript: 'tree-sitter-typescript',
      tsx: 'tree-sitter-tsx',
      rust: 'tree-sitter-rust',
      go: 'tree-sitter-go',
      java: 'tree-sitter-java',
      cpp: 'tree-sitter-cpp',
      c: 'tree-sitter-c',
    };

    const wasmName = languageMap[language];
    if (!wasmName) {
      return null;
    }

    try {
      const parser = new this.Parser();
      // Note: In production, WASM files should be pre-bundled
      // For now, we'll catch the error and fall back
      const Lang = await this.Parser.Language.load(`/wasm/${wasmName}.wasm`);
      parser.setLanguage(Lang);
      this.parsers.set(language, parser);
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

    const langPatterns = patterns[language] || patterns.javascript || [];
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
   */
  async chunkWithTreeSitter(
    filePath: string,
    content: string,
  ): Promise<TreeSitterChunk[]> {
    const ext = path.extname(filePath);
    const language = getLanguageFromExtension(ext);
    const lines = content.split('\n');

    // Skip if file too large
    if (content.length > this.MAX_FILE_SIZE) {
      this.logger.debug(`File too large for AST parsing: ${filePath}`);
      return this.chunkByLines(filePath, content, language);
    }

    // Try to get parser
    const parser = await this.getParser(language);
    if (!parser) {
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
