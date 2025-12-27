import { Injectable, Logger } from '@nestjs/common';
import * as path from 'path';
import { getLanguageFromExtension } from '../config/configuration';

export interface CodeChunk {
  doc_id: string;
  path: string;
  language: string;
  symbols: string[];
  content: string;
  start_line: number;
  end_line: number;
  chunk_index: number;
}

export interface ChunkingOptions {
  maxChunkLines?: number;
  overlapLines?: number;
  extractSymbols?: boolean;
}

@Injectable()
export class ChunkerService {
  private readonly logger = new Logger(ChunkerService.name);
  private readonly DEFAULT_MAX_CHUNK_LINES = 50;
  private readonly DEFAULT_OVERLAP_LINES = 5;

  /**
   * Chunk code content into searchable pieces
   * @param filePath File path
   * @param content File content
   * @param options Chunking options
   * @returns Array of code chunks
   */
  chunkCode(
    filePath: string,
    content: string,
    options: ChunkingOptions = {},
  ): CodeChunk[] {
    const {
      maxChunkLines = this.DEFAULT_MAX_CHUNK_LINES,
      overlapLines = this.DEFAULT_OVERLAP_LINES,
      extractSymbols = true,
    } = options;

    const ext = path.extname(filePath);
    const language = getLanguageFromExtension(ext);
    const lines = content.split('\n');

    // For small files, return as single chunk
    if (lines.length <= maxChunkLines) {
      const symbols = extractSymbols
        ? this.extractSymbolsFromContent(content, language)
        : [];

      return [
        {
          doc_id: this.generateDocId(filePath, 0, content),
          path: filePath,
          language,
          symbols,
          content,
          start_line: 1,
          end_line: lines.length,
          chunk_index: 0,
        },
      ];
    }

    // Chunk larger files with overlap
    const chunks: CodeChunk[] = [];
    let chunkIndex = 0;
    let startLine = 0;

    while (startLine < lines.length) {
      const endLine = Math.min(startLine + maxChunkLines, lines.length);
      const chunkLines = lines.slice(startLine, endLine);
      const chunkContent = chunkLines.join('\n');

      const symbols = extractSymbols
        ? this.extractSymbolsFromContent(chunkContent, language)
        : [];

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
      startLine = endLine - overlapLines;

      // Avoid infinite loop
      if (startLine >= lines.length - overlapLines) {
        break;
      }
    }

    return chunks;
  }

  /**
   * Extract symbol names from code content
   * Uses simple regex patterns for common constructs
   */
  private extractSymbolsFromContent(
    content: string,
    language: string,
  ): string[] {
    const symbols: Set<string> = new Set();

    // Language-specific patterns
    const patterns: Record<string, RegExp[]> = {
      python: [
        /def\s+(\w+)\s*\(/g, // function definitions
        /class\s+(\w+)\s*[:(]/g, // class definitions
        /(\w+)\s*=/g, // variable assignments (simplified)
      ],
      javascript: [
        /function\s+(\w+)\s*\(/g, // function declarations
        /const\s+(\w+)\s*=/g, // const declarations
        /let\s+(\w+)\s*=/g, // let declarations
        /class\s+(\w+)\s*[{extends]/g, // class declarations
        /(\w+)\s*:\s*function/g, // object methods
        /(\w+)\s*\([^)]*\)\s*{/g, // arrow functions (simplified)
      ],
      typescript: [
        /function\s+(\w+)\s*[<(]/g, // function declarations
        /const\s+(\w+)\s*[=:]/g, // const declarations
        /let\s+(\w+)\s*[=:]/g, // let declarations
        /class\s+(\w+)\s*[{<extends implements]/g, // class declarations
        /interface\s+(\w+)\s*[{<]/g, // interface declarations
        /type\s+(\w+)\s*[=<]/g, // type declarations
        /(\w+)\s*\([^)]*\)\s*[:{]/g, // methods
      ],
      rust: [
        /fn\s+(\w+)\s*[<(]/g, // function definitions
        /struct\s+(\w+)\s*[{<]/g, // struct definitions
        /enum\s+(\w+)\s*[{<]/g, // enum definitions
        /trait\s+(\w+)\s*[{<]/g, // trait definitions
        /impl\s+(\w+)/g, // impl blocks
      ],
      go: [
        /func\s+(\w+)\s*\(/g, // function declarations
        /func\s*\(\w+\s+\*?\w+\)\s+(\w+)\s*\(/g, // method declarations
        /type\s+(\w+)\s+struct/g, // struct definitions
        /type\s+(\w+)\s+interface/g, // interface definitions
      ],
      java: [
        /class\s+(\w+)\s*[{<extends implements]/g, // class declarations
        /interface\s+(\w+)\s*[{<]/g, // interface declarations
        /(?:public|private|protected)?\s*(?:static)?\s*\w+\s+(\w+)\s*\(/g, // method declarations
      ],
    };

    const langPatterns = patterns[language] || patterns.javascript;

    for (const pattern of langPatterns) {
      let match;
      const regex = new RegExp(pattern.source, pattern.flags);
      while ((match = regex.exec(content)) !== null) {
        if (match[1] && match[1].length > 1 && match[1].length < 50) {
          // Filter out common keywords
          const keywords = new Set([
            'if',
            'else',
            'for',
            'while',
            'return',
            'import',
            'from',
            'export',
            'default',
            'const',
            'let',
            'var',
            'function',
            'class',
            'interface',
            'type',
            'struct',
            'enum',
          ]);
          if (!keywords.has(match[1].toLowerCase())) {
            symbols.add(match[1]);
          }
        }
      }
    }

    return Array.from(symbols);
  }

  /**
   * Generate stable document ID
   */
  private generateDocId(
    filePath: string,
    chunkIndex: number,
    content: string,
  ): string {
    // Use a simple hash based on path, index, and content length
    const hashInput = `${filePath}:${chunkIndex}:${content.length}`;
    let hash = 0;
    for (let i = 0; i < hashInput.length; i++) {
      const char = hashInput.charCodeAt(i);
      hash = (hash << 5) - hash + char;
      hash = hash & hash; // Convert to 32-bit integer
    }
    return `${filePath}#${chunkIndex}#${Math.abs(hash).toString(16)}`;
  }

  /**
   * Estimate if file is likely binary
   */
  isBinary(content: string): boolean {
    // Check for null bytes
    if (content.includes('\0')) {
      return true;
    }

    // Check ratio of non-printable characters
    let nonPrintable = 0;
    const sampleSize = Math.min(content.length, 8000);

    for (let i = 0; i < sampleSize; i++) {
      const code = content.charCodeAt(i);
      if (
        code < 32 &&
        code !== 9 &&
        code !== 10 &&
        code !== 13 // Not tab, LF, CR
      ) {
        nonPrintable++;
      }
    }

    return nonPrintable / sampleSize > 0.1;
  }
}
