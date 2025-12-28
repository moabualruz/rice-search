import { Injectable, Logger, OnModuleInit } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";

/**
 * Sparse vector representation
 * Uses parallel arrays for efficiency (indices/values)
 */
export interface SparseVector {
  indices: number[];   // Vocabulary term IDs
  values: number[];    // Learned weights (non-zero only)
  dimension?: number;  // Vocabulary size (optional)
}

/**
 * Sparse encoding result with metadata
 */
export interface SparseEncodingResult {
  vector: SparseVector;
  latencyMs: number;
  model: string;
  tokenCount: number;
}

/**
 * Sparse encoder configuration
 */
export interface SparseEncoderConfig {
  model: "splade-v2-distil" | "stub" | "custom";
  serviceUrl?: string;
  topK: number;        // Keep top K terms per document
  minWeight: number;   // Minimum weight threshold
  maxTokens: number;   // Maximum input tokens
}

/**
 * SparseEncoderService provides learned sparse vector encoding.
 *
 * This is an abstraction layer that supports multiple backends:
 * - "stub": Simple term-frequency based vectors (default, no external deps)
 * - "splade-v2-distil": SPLADE model via HTTP service
 *
 * The stub implementation provides a working baseline while the actual
 * learned sparse models can be deployed as separate services.
 *
 * Why learned sparse matters:
 * - BM25 uses statistical term weights (TF-IDF)
 * - SPLADE learns term weights that capture semantics
 * - Query expansion happens automatically in the model
 * - Better handling of synonyms and paraphrases
 */
@Injectable()
export class SparseEncoderService implements OnModuleInit {
  private readonly logger = new Logger(SparseEncoderService.name);
  private readonly config: SparseEncoderConfig;
  private vocabulary: Map<string, number> = new Map();
  private nextVocabId = 0;

  constructor(private configService: ConfigService) {
    this.config = {
      model: this.configService.get<string>("sparse.model") as SparseEncoderConfig["model"] ?? "stub",
      serviceUrl: this.configService.get<string>("sparse.serviceUrl"),
      topK: this.configService.get<number>("sparse.topK") ?? 128,
      minWeight: this.configService.get<number>("sparse.minWeight") ?? 0.01,
      maxTokens: this.configService.get<number>("sparse.maxTokens") ?? 512,
    };
  }

  async onModuleInit(): Promise<void> {
    this.logger.log(
      `Sparse encoder initialized: model=${this.config.model}, topK=${this.config.topK}`,
    );

    if (this.config.model !== "stub" && this.config.serviceUrl) {
      // Verify external service is available
      try {
        const response = await fetch(`${this.config.serviceUrl}/health`);
        if (response.ok) {
          this.logger.log(`Connected to sparse encoder service: ${this.config.serviceUrl}`);
        }
      } catch {
        this.logger.warn(
          `Sparse encoder service not available at ${this.config.serviceUrl}, falling back to stub`,
        );
        (this.config as SparseEncoderConfig).model = "stub";
      }
    }
  }

  /**
   * Encode text to sparse vector
   *
   * @param text - Input text
   * @returns Sparse encoding result
   */
  async encode(text: string): Promise<SparseEncodingResult> {
    const startTime = Date.now();

    let vector: SparseVector;
    let tokenCount: number;

    switch (this.config.model) {
      case "splade-v2-distil":
        ({ vector, tokenCount } = await this.encodeWithService(text));
        break;
      case "stub":
      default:
        ({ vector, tokenCount } = this.encodeStub(text));
        break;
    }

    return {
      vector,
      latencyMs: Date.now() - startTime,
      model: this.config.model,
      tokenCount,
    };
  }

  /**
   * Batch encode texts for indexing
   *
   * @param texts - Input texts
   * @returns Array of sparse encoding results
   */
  async encodeBatch(texts: string[]): Promise<SparseEncodingResult[]> {
    const startTime = Date.now();

    if (this.config.model === "stub") {
      // Stub can process in parallel locally
      return texts.map((text) => {
        const { vector, tokenCount } = this.encodeStub(text);
        return {
          vector,
          latencyMs: 0,
          model: this.config.model,
          tokenCount,
        };
      });
    }

    // For real models, batch through service
    const results = await this.encodeBatchWithService(texts);

    const latency = Date.now() - startTime;
    this.logger.debug(`Batch encoded ${texts.length} texts in ${latency}ms`);

    return results;
  }

  /**
   * Encode query with potential expansion
   * Query encoding may use different strategy than document encoding
   *
   * @param query - Search query
   * @returns Sparse encoding result
   */
  async encodeQuery(query: string): Promise<SparseEncodingResult> {
    // For now, same as regular encode
    // Real SPLADE models have separate query encoders
    return this.encode(query);
  }

  /**
   * Compute sparse dot product similarity
   *
   * @param a - First sparse vector
   * @param b - Second sparse vector
   * @returns Similarity score
   */
  computeSimilarity(a: SparseVector, b: SparseVector): number {
    // Build map from b for O(1) lookup
    const bMap = new Map<number, number>();
    for (let i = 0; i < b.indices.length; i++) {
      bMap.set(b.indices[i], b.values[i]);
    }

    // Compute dot product over shared indices
    let score = 0;
    for (let i = 0; i < a.indices.length; i++) {
      const bVal = bMap.get(a.indices[i]);
      if (bVal !== undefined) {
        score += a.values[i] * bVal;
      }
    }

    return score;
  }

  /**
   * Get configuration info
   */
  getConfig(): SparseEncoderConfig {
    return { ...this.config };
  }

  /**
   * Stub implementation using term frequency
   * This provides a working baseline without external dependencies
   */
  private encodeStub(text: string): { vector: SparseVector; tokenCount: number } {
    // Tokenize
    const tokens = this.tokenize(text);
    const tokenCount = tokens.length;

    // Count term frequencies
    const termCounts = new Map<string, number>();
    for (const token of tokens) {
      termCounts.set(token, (termCounts.get(token) ?? 0) + 1);
    }

    // Convert to sparse vector
    const indices: number[] = [];
    const values: number[] = [];

    for (const [term, count] of termCounts) {
      // Get or create vocabulary ID
      let vocabId = this.vocabulary.get(term);
      if (vocabId === undefined) {
        vocabId = this.nextVocabId++;
        this.vocabulary.set(term, vocabId);
      }

      // Use log-scaled term frequency as weight
      const weight = 1 + Math.log(count);
      if (weight >= this.config.minWeight) {
        indices.push(vocabId);
        values.push(weight);
      }
    }

    // Sort by weight descending and take top K
    const pairs = indices.map((idx, i) => ({ idx, val: values[i] }));
    pairs.sort((a, b) => b.val - a.val);
    const topK = pairs.slice(0, this.config.topK);

    return {
      vector: {
        indices: topK.map((p) => p.idx),
        values: topK.map((p) => p.val),
        dimension: this.nextVocabId,
      },
      tokenCount,
    };
  }

  /**
   * Tokenize text for stub encoder
   */
  private tokenize(text: string): string[] {
    return text
      .toLowerCase()
      // Split camelCase
      .replace(/([a-z])([A-Z])/g, "$1 $2")
      // Split on non-alphanumeric
      .split(/[^a-z0-9]+/)
      .filter((t) => t.length >= 2 && t.length <= 30);
  }

  /**
   * Encode using external service
   */
  private async encodeWithService(text: string): Promise<{ vector: SparseVector; tokenCount: number }> {
    if (!this.config.serviceUrl) {
      throw new Error("Sparse encoder service URL not configured");
    }

    const response = await fetch(`${this.config.serviceUrl}/encode`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text, top_k: this.config.topK }),
    });

    if (!response.ok) {
      throw new Error(`Sparse encoder service error: ${response.statusText}`);
    }

    const result = await response.json() as {
      indices: number[];
      values: number[];
      token_count: number;
    };

    return {
      vector: {
        indices: result.indices,
        values: result.values,
      },
      tokenCount: result.token_count,
    };
  }

  /**
   * Batch encode using external service
   */
  private async encodeBatchWithService(texts: string[]): Promise<SparseEncodingResult[]> {
    if (!this.config.serviceUrl) {
      throw new Error("Sparse encoder service URL not configured");
    }

    const response = await fetch(`${this.config.serviceUrl}/encode_batch`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ texts, top_k: this.config.topK }),
    });

    if (!response.ok) {
      throw new Error(`Sparse encoder service error: ${response.statusText}`);
    }

    const results = await response.json() as Array<{
      indices: number[];
      values: number[];
      token_count: number;
    }>;

    return results.map((r) => ({
      vector: {
        indices: r.indices,
        values: r.values,
      },
      latencyMs: 0,
      model: this.config.model,
      tokenCount: r.token_count,
    }));
  }
}
