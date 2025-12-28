import { Injectable, Logger } from "@nestjs/common";
import { QueryClassifierService, QueryClassification } from "../services/query-classifier.service";

export type QueryIntent =
  | "navigational"  // Exact lookup: file, symbol, line
  | "factual"       // Specific answer: "what does X do"
  | "exploratory"   // Broad search: "how does auth work"
  | "analytical";   // Deep understanding: "compare X and Y"

export type QueryDifficulty = "easy" | "medium" | "hard";

export interface IntentSignals {
  queryLength: number;
  specificity: number;
  hasExactTarget: boolean;
  requiresContext: boolean;
  isComparison: boolean;
}

export interface IntentClassification {
  intent: QueryIntent;
  confidence: number;  // 0-1
  difficulty: QueryDifficulty;
  signals: IntentSignals;
  baseClassification: QueryClassification;
}

@Injectable()
export class IntentClassifierService {
  private readonly logger = new Logger(IntentClassifierService.name);

  // Comparison patterns for analytical queries
  private readonly COMPARISON_PATTERNS = [
    /\bvs\.?\b/i,
    /\bversus\b/i,
    /\bcompare\b/i,
    /\bcomparison\b/i,
    /\bdifference\s+between\b/i,
    /\bor\b.*\bwhich\b/i,
    /\bbetter\b/i,
    /\bprefer\b/i,
    /\badvantage\b/i,
    /\bdisadvantage\b/i,
    /\bpros\b.*\bcons\b/i,
    /\btrade.?off\b/i,
  ];

  // Factual query patterns
  private readonly FACTUAL_PATTERNS = [
    /^what\s+(is|does|are)\b/i,
    /^how\s+does\b/i,
    /^where\s+(is|are|does)\b/i,
    /^when\s+(is|does|did)\b/i,
    /^which\s+\w+\s+(is|does|has)\b/i,
    /\breturn\s+type\b/i,
    /\btype\s+of\b/i,
    /\bdefinition\s+of\b/i,
  ];

  // Context-requiring patterns (needs understanding of surrounding code)
  private readonly CONTEXT_PATTERNS = [
    /\bhow\s+does\s+.+\s+work\b/i,
    /\bexplain\b/i,
    /\bdescribe\b/i,
    /\bunderstand\b/i,
    /\bflow\b/i,
    /\barchitecture\b/i,
    /\bpattern\b/i,
    /\bdesign\b/i,
  ];

  constructor(private queryClassifier: QueryClassifierService) {}

  /**
   * Classify query intent and estimate difficulty
   * @param query The normalized search query
   * @returns Full intent classification with signals
   */
  classify(query: string): IntentClassification {
    const baseClassification = this.queryClassifier.classify(query);
    const signals = this.extractSignals(query, baseClassification);
    const intent = this.detectIntent(query, baseClassification, signals);
    const confidence = this.calculateConfidence(intent, signals, baseClassification);
    const difficulty = this.estimateDifficulty(intent, signals, baseClassification);

    this.logger.debug(
      `Intent: ${intent} (confidence: ${confidence.toFixed(2)}, difficulty: ${difficulty}): "${query.substring(0, 50)}..."`
    );

    return {
      intent,
      confidence,
      difficulty,
      signals,
      baseClassification,
    };
  }

  /**
   * Extract intent-specific signals from query
   */
  private extractSignals(query: string, base: QueryClassification): IntentSignals {
    return {
      queryLength: query.length,
      specificity: base.signals.specificity,
      hasExactTarget: this.hasExactTarget(query, base),
      requiresContext: this.requiresContext(query),
      isComparison: this.hasComparisonPattern(query),
    };
  }

  /**
   * Detect primary intent based on query and base classification
   */
  private detectIntent(
    query: string,
    base: QueryClassification,
    signals: IntentSignals,
  ): QueryIntent {
    // Navigational: short + exact target (file path, symbol name)
    // Priority check - if it looks like a file/symbol lookup, it's navigational
    if (base.signals.isNavigational) {
      return "navigational";
    }

    if (
      base.signals.hasPathPatterns ||
      (base.signals.wordCount <= 2 && base.signals.hasCamelCase)
    ) {
      return "navigational";
    }

    // Analytical: comparison patterns, multiple subjects
    if (signals.isComparison) {
      return "analytical";
    }

    // Factual: question word + specific subject + high specificity
    if (base.signals.hasQuestionWords && base.signals.specificity > 0.5) {
      // Check for factual patterns
      if (this.hasFactualPattern(query)) {
        return "factual";
      }
    }

    // Exploratory: broad concept search, requires context
    if (base.signals.isExploratory || signals.requiresContext) {
      return "exploratory";
    }

    // Natural language with low specificity → exploratory
    if (base.type === "natural" && base.signals.specificity < 0.4) {
      return "exploratory";
    }

    // Default: factual for short queries, exploratory for longer
    if (base.signals.wordCount <= 4) {
      return "factual";
    }

    return "exploratory";
  }

  /**
   * Calculate confidence in the intent classification
   */
  private calculateConfidence(
    intent: QueryIntent,
    signals: IntentSignals,
    base: QueryClassification,
  ): number {
    let confidence = 0.5; // Start neutral

    switch (intent) {
      case "navigational":
        // High confidence if exact target found
        if (signals.hasExactTarget) confidence += 0.3;
        if (base.signals.hasPathPatterns) confidence += 0.15;
        if (base.signals.hasCamelCase && base.signals.wordCount <= 2) confidence += 0.15;
        break;

      case "factual":
        // Higher confidence with specific question patterns
        if (base.signals.hasQuestionWords) confidence += 0.2;
        if (signals.specificity > 0.6) confidence += 0.15;
        if (base.signals.wordCount <= 6) confidence += 0.1;
        break;

      case "exploratory":
        // Higher confidence with broad context patterns
        if (signals.requiresContext) confidence += 0.2;
        if (base.signals.isExploratory) confidence += 0.15;
        if (base.signals.wordCount >= 5) confidence += 0.1;
        break;

      case "analytical":
        // High confidence if comparison pattern found
        if (signals.isComparison) confidence += 0.35;
        break;
    }

    // Clamp to [0, 1]
    return Math.max(0, Math.min(1, confidence));
  }

  /**
   * Estimate query difficulty based on intent and signals
   */
  private estimateDifficulty(
    intent: QueryIntent,
    signals: IntentSignals,
    base: QueryClassification,
  ): QueryDifficulty {
    // Navigational: always easy (exact lookup)
    if (intent === "navigational") {
      return "easy";
    }

    // Analytical: always hard (requires deep understanding)
    if (intent === "analytical") {
      return "hard";
    }

    // Factual: depends on specificity
    if (intent === "factual") {
      if (signals.specificity >= 0.7) return "easy";
      if (signals.specificity >= 0.4) return "medium";
      return "hard";
    }

    // Exploratory: depends on context requirements and query length
    if (intent === "exploratory") {
      if (signals.requiresContext) return "hard";
      if (base.signals.wordCount >= 8) return "hard";
      if (base.signals.wordCount >= 5) return "medium";
      return "medium";
    }

    return "medium";
  }

  /**
   * Check if query has an exact target (file path, specific symbol)
   */
  private hasExactTarget(query: string, base: QueryClassification): boolean {
    // File path is an exact target
    if (base.signals.hasPathPatterns) return true;

    // Single CamelCase word is likely a symbol lookup
    if (base.signals.hasCamelCase && base.signals.wordCount === 1) return true;

    // Contains file extension reference
    if (/\.\w{1,4}\b/.test(query)) return true;

    // Contains line number reference
    if (/\bline\s*\d+\b/i.test(query)) return true;

    return false;
  }

  /**
   * Check if query requires understanding of context
   */
  private requiresContext(query: string): boolean {
    return this.CONTEXT_PATTERNS.some((pattern) => pattern.test(query));
  }

  /**
   * Check if query contains comparison patterns
   */
  private hasComparisonPattern(query: string): boolean {
    return this.COMPARISON_PATTERNS.some((pattern) => pattern.test(query));
  }

  /**
   * Check if query matches factual patterns
   */
  private hasFactualPattern(query: string): boolean {
    return this.FACTUAL_PATTERNS.some((pattern) => pattern.test(query));
  }
}
