import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { IntentClassification, QueryIntent, QueryDifficulty } from "./intent-classifier.service";

export type RetrievalStrategy =
  | "sparse-only"    // BM25 only, skip dense (navigational)
  | "dense-heavy"    // Favor semantic (exploratory)
  | "balanced"       // Standard hybrid (factual)
  | "deep-rerank";   // Expanded candidates + multi-pass (analytical)

export interface RetrievalConfig {
  strategy: RetrievalStrategy;
  sparseTopK: number;
  denseTopK: number;
  sparseWeight: number;
  denseWeight: number;
  rerankCandidates: number;
  useSecondPass: boolean;
  secondPassCandidates: number;
}

// Strategy presets (base configurations before difficulty adjustment)
const STRATEGY_PRESETS: Record<RetrievalStrategy, Omit<RetrievalConfig, "strategy">> = {
  "sparse-only": {
    sparseTopK: 50,
    denseTopK: 0,
    sparseWeight: 1.0,
    denseWeight: 0.0,
    rerankCandidates: 10,
    useSecondPass: false,
    secondPassCandidates: 0,
  },
  "balanced": {
    sparseTopK: 80,
    denseTopK: 80,
    sparseWeight: 0.5,
    denseWeight: 0.5,
    rerankCandidates: 30,
    useSecondPass: false,
    secondPassCandidates: 0,
  },
  "dense-heavy": {
    sparseTopK: 60,
    denseTopK: 120,
    sparseWeight: 0.3,
    denseWeight: 0.7,
    rerankCandidates: 50,
    useSecondPass: false,
    secondPassCandidates: 20,
  },
  "deep-rerank": {
    sparseTopK: 150,
    denseTopK: 150,
    sparseWeight: 0.4,
    denseWeight: 0.6,
    rerankCandidates: 100,
    useSecondPass: true,
    secondPassCandidates: 30,
  },
};

@Injectable()
export class StrategySelectorService {
  private readonly logger = new Logger(StrategySelectorService.name);

  // Configurable limits
  private readonly maxSparseTopK: number;
  private readonly maxDenseTopK: number;
  private readonly maxRerankCandidates: number;

  constructor(private configService: ConfigService) {
    this.maxSparseTopK = this.configService.get<number>("search.maxSparseTopK") ?? 300;
    this.maxDenseTopK = this.configService.get<number>("search.maxDenseTopK") ?? 300;
    this.maxRerankCandidates = this.configService.get<number>("rerank.maxCandidates") ?? 150;
  }

  /**
   * Select retrieval strategy based on intent classification
   * @param intent The intent classification from IntentClassifierService
   * @returns Retrieval configuration
   */
  selectStrategy(intent: IntentClassification): RetrievalConfig {
    const strategy = this.intentToStrategy(intent.intent, intent.difficulty);
    const baseConfig = this.getBaseConfig(strategy);

    this.logger.debug(
      `Selected strategy: ${strategy} for intent ${intent.intent} (difficulty: ${intent.difficulty})`
    );

    return baseConfig;
  }

  /**
   * Map intent to retrieval strategy
   */
  private intentToStrategy(intent: QueryIntent, difficulty: QueryDifficulty): RetrievalStrategy {
    switch (intent) {
      case "navigational":
        // Always sparse-only for navigational (exact lookup)
        return "sparse-only";

      case "factual":
        // Balanced hybrid for factual queries
        return "balanced";

      case "exploratory":
        // Dense-heavy for concept search, with second pass for hard queries
        return "dense-heavy";

      case "analytical":
        // Deep rerank for complex understanding queries
        return "deep-rerank";

      default:
        return "balanced";
    }
  }

  /**
   * Get base configuration for a strategy
   */
  private getBaseConfig(strategy: RetrievalStrategy): RetrievalConfig {
    const preset = STRATEGY_PRESETS[strategy];
    return {
      strategy,
      ...preset,
    };
  }

  /**
   * Adjust candidate counts based on query difficulty (Task 1.3)
   * Easy → reduce candidates for speed
   * Hard → expand candidates for thoroughness
   */
  adjustCandidates(
    config: RetrievalConfig,
    intent: IntentClassification,
  ): RetrievalConfig {
    const { difficulty } = intent;

    // Easy queries: reduce candidates for speed
    if (difficulty === "easy") {
      return {
        ...config,
        sparseTopK: Math.min(config.sparseTopK, 50),
        denseTopK: Math.min(config.denseTopK, 50),
        rerankCandidates: Math.min(config.rerankCandidates, 20),
        useSecondPass: false, // Never second pass for easy
        secondPassCandidates: 0,
      };
    }

    // Hard queries: expand candidates
    if (difficulty === "hard") {
      return {
        ...config,
        sparseTopK: Math.min(config.sparseTopK * 1.5, this.maxSparseTopK),
        denseTopK: Math.min(config.denseTopK * 1.5, this.maxDenseTopK),
        rerankCandidates: Math.min(config.rerankCandidates * 1.5, this.maxRerankCandidates),
        useSecondPass: config.strategy !== "sparse-only", // Enable second pass for hard queries
        secondPassCandidates: Math.min(config.secondPassCandidates || 20, 40),
      };
    }

    // Medium: use base config as-is
    return config;
  }

  /**
   * Apply user overrides to strategy config
   * User-specified values take precedence over strategy defaults
   */
  applyOverrides(
    config: RetrievalConfig,
    overrides: {
      sparseWeight?: number;
      denseWeight?: number;
      rerankCandidates?: number;
      enableReranking?: boolean;
    },
  ): RetrievalConfig {
    const result = { ...config };

    if (overrides.sparseWeight !== undefined) {
      result.sparseWeight = overrides.sparseWeight;
    }
    if (overrides.denseWeight !== undefined) {
      result.denseWeight = overrides.denseWeight;
    }
    if (overrides.rerankCandidates !== undefined) {
      result.rerankCandidates = overrides.rerankCandidates;
    }
    if (overrides.enableReranking === false) {
      result.rerankCandidates = 0;
      result.useSecondPass = false;
    }

    return result;
  }
}
