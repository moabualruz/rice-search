import { Injectable, Logger } from "@nestjs/common";

export type QueryType = "code" | "natural" | "hybrid";

export interface QueryClassification {
  type: QueryType;
  confidence: number; // 0-1
  signals: {
    hasCodePatterns: boolean;
    hasPathPatterns: boolean;
    hasCamelCase: boolean;
    wordCount: number;
    avgWordLength: number;
  };
}

@Injectable()
export class QueryClassifierService {
  private readonly logger = new Logger(QueryClassifierService.name);

  // Code-specific symbols that strongly indicate code queries
  private readonly CODE_SYMBOLS = new Set([
    "(",
    ")",
    "{",
    "}",
    "[",
    "]",
    ".",
    ":",
    ";",
    "=",
    "<",
    ">",
    "!",
    "&",
    "|",
    "=>",
    "->",
  ]);

  // Programming language keywords
  private readonly CODE_KEYWORDS = new Set([
    "function",
    "class",
    "def",
    "const",
    "let",
    "var",
    "import",
    "export",
    "async",
    "await",
    "return",
    "if",
    "else",
    "for",
    "while",
    "switch",
    "case",
    "break",
    "continue",
    "try",
    "catch",
    "throw",
    "new",
    "this",
    "self",
    "super",
    "static",
    "public",
    "private",
    "protected",
    "interface",
    "type",
    "enum",
    "struct",
    "impl",
    "trait",
    "fn",
    "func",
    "package",
    "namespace",
  ]);

  // Common file extensions
  private readonly FILE_EXTENSIONS = new Set([
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".py",
    ".rs",
    ".go",
    ".java",
    ".c",
    ".cpp",
    ".h",
    ".hpp",
    ".cs",
    ".rb",
    ".php",
    ".swift",
    ".kt",
    ".scala",
    ".sh",
    ".bash",
    ".yml",
    ".yaml",
    ".json",
    ".xml",
    ".html",
    ".css",
    ".scss",
    ".md",
  ]);

  // Natural language query starters
  private readonly NL_STARTERS = new Set([
    "how",
    "what",
    "where",
    "why",
    "when",
    "which",
    "who",
    "can",
    "should",
    "is",
    "are",
    "does",
    "do",
    "find",
    "show",
    "get",
    "list",
    "search",
    "explain",
    "describe",
  ]);

  /**
   * Classify a query as code, natural language, or hybrid
   * @param query The search query to classify
   * @returns QueryClassification with type, confidence, and signals
   */
  classify(query: string): QueryClassification {
    const trimmedQuery = query.trim();

    if (!trimmedQuery) {
      return {
        type: "natural",
        confidence: 0,
        signals: {
          hasCodePatterns: false,
          hasPathPatterns: false,
          hasCamelCase: false,
          wordCount: 0,
          avgWordLength: 0,
        },
      };
    }

    // Extract signals
    const signals = this.extractSignals(trimmedQuery);

    // Calculate code score (0-1)
    const codeScore = this.calculateCodeScore(trimmedQuery, signals);

    // Determine type based on thresholds
    let type: QueryType;
    let confidence: number;

    if (codeScore >= 0.6) {
      type = "code";
      confidence = codeScore;
    } else if (codeScore <= 0.3) {
      type = "natural";
      confidence = 1 - codeScore;
    } else {
      type = "hybrid";
      confidence = 1 - Math.abs(codeScore - 0.5) * 2; // Closer to 0.5 = higher hybrid confidence
    }

    this.logger.debug(
      `Query classified as ${type} (confidence: ${confidence.toFixed(2)}, code_score: ${codeScore.toFixed(2)}): "${trimmedQuery.substring(0, 50)}..."`,
    );

    return {
      type,
      confidence,
      signals,
    };
  }

  /**
   * Extract classification signals from query
   */
  private extractSignals(query: string): QueryClassification["signals"] {
    const words = this.tokenizeQuery(query);

    return {
      hasCodePatterns: this.hasCodeSymbols(query) || this.hasCodeKeywords(query),
      hasPathPatterns: this.hasPathPattern(query),
      hasCamelCase: this.hasCamelCase(query),
      wordCount: words.length,
      avgWordLength:
        words.length > 0
          ? words.reduce((sum, w) => sum + w.length, 0) / words.length
          : 0,
    };
  }

  /**
   * Calculate code score (0 = definitely natural language, 1 = definitely code)
   */
  private calculateCodeScore(
    query: string,
    signals: QueryClassification["signals"],
  ): number {
    let score = 0.5; // Start neutral

    // Symbol presence (+0.2)
    if (this.hasCodeSymbols(query)) {
      const symbolDensity = this.calculateSymbolDensity(query);
      score += Math.min(0.2, symbolDensity * 0.4);
    }

    // Code keywords (+0.15)
    if (this.hasCodeKeywords(query)) {
      const keywordCount = this.countCodeKeywords(query);
      score += Math.min(0.15, keywordCount * 0.05);
    }

    // File extensions (+0.15)
    if (this.hasFileExtension(query)) {
      score += 0.15;
    }

    // Path patterns (+0.15)
    if (signals.hasPathPatterns) {
      score += 0.15;
    }

    // CamelCase or snake_case (+0.1)
    if (signals.hasCamelCase || this.hasSnakeCase(query)) {
      score += 0.1;
    }

    // Short query (1-3 words) tends to be code (+0.1)
    if (signals.wordCount >= 1 && signals.wordCount <= 3) {
      score += 0.1;
    }

    // Natural language indicators (negative adjustments)

    // Starts with question word (-0.2)
    if (this.startsWithNaturalLanguage(query)) {
      score -= 0.2;
    }

    // Long query (5+ words) tends to be natural (-0.15)
    if (signals.wordCount >= 5) {
      score -= 0.15;
    }

    // No symbols or keywords (-0.1)
    if (!signals.hasCodePatterns && !signals.hasPathPatterns && !signals.hasCamelCase) {
      score -= 0.1;
    }

    // Contains common verbs (-0.1)
    if (this.hasCommonVerbs(query)) {
      score -= 0.1;
    }

    // Clamp to [0, 1]
    return Math.max(0, Math.min(1, score));
  }

  /**
   * Check if query contains code symbols
   */
  private hasCodeSymbols(query: string): boolean {
    for (const symbol of this.CODE_SYMBOLS) {
      if (query.includes(symbol)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Calculate density of code symbols in query
   */
  private calculateSymbolDensity(query: string): number {
    let symbolCount = 0;
    for (const char of query) {
      if (this.CODE_SYMBOLS.has(char)) {
        symbolCount++;
      }
    }
    return symbolCount / Math.max(query.length, 1);
  }

  /**
   * Check if query contains programming keywords
   */
  private hasCodeKeywords(query: string): boolean {
    const words = this.tokenizeQuery(query);
    return words.some((word) => this.CODE_KEYWORDS.has(word.toLowerCase()));
  }

  /**
   * Count number of code keywords
   */
  private countCodeKeywords(query: string): number {
    const words = this.tokenizeQuery(query);
    return words.filter((word) => this.CODE_KEYWORDS.has(word.toLowerCase())).length;
  }

  /**
   * Check if query contains file extensions
   */
  private hasFileExtension(query: string): boolean {
    const lowerQuery = query.toLowerCase();
    for (const ext of this.FILE_EXTENSIONS) {
      if (lowerQuery.includes(ext)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Check if query contains path patterns like /foo/bar or \src\index
   */
  private hasPathPattern(query: string): boolean {
    // Unix-style paths: /foo/bar.ts
    const unixPath = /\/[\w\-_.]+(?:\/[\w\-_.]+)+/;
    // Windows-style paths: \src\index.js
    const windowsPath = /\\[\w\-_.]+(?:\\[\w\-_.]+)+/;
    // Relative paths: ./foo or ../bar
    const relativePath = /\.\.?\/[\w\-_.]+/;

    return (
      unixPath.test(query) || windowsPath.test(query) || relativePath.test(query)
    );
  }

  /**
   * Check if query contains camelCase identifiers
   */
  private hasCamelCase(query: string): boolean {
    // Match camelCase or PascalCase (word with internal capitals)
    const camelCasePattern = /\b[a-z]+[A-Z][a-zA-Z]*\b|\b[A-Z][a-z]+[A-Z][a-zA-Z]*\b/;
    return camelCasePattern.test(query);
  }

  /**
   * Check if query contains snake_case identifiers
   */
  private hasSnakeCase(query: string): boolean {
    const snakeCasePattern = /\b[a-z]+_[a-z_]+\b/;
    return snakeCasePattern.test(query);
  }

  /**
   * Check if query starts with natural language words
   */
  private startsWithNaturalLanguage(query: string): boolean {
    const firstWord = query.trim().split(/\s+/)[0]?.toLowerCase();
    return firstWord ? this.NL_STARTERS.has(firstWord) : false;
  }

  /**
   * Check if query contains common verbs
   */
  private hasCommonVerbs(query: string): boolean {
    const verbs = new Set([
      "find",
      "search",
      "show",
      "get",
      "list",
      "display",
      "fetch",
      "retrieve",
      "locate",
    ]);
    const words = this.tokenizeQuery(query);
    return words.some((word) => verbs.has(word.toLowerCase()));
  }

  /**
   * Tokenize query into words
   */
  private tokenizeQuery(query: string): string[] {
    return query
      .split(/[\s\-_.,;:!?()\[\]{}'"]+/)
      .filter((token) => token.length > 0);
  }
}
