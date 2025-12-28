import { Injectable, Logger } from "@nestjs/common";
import { HybridSearchResult } from "../services/hybrid-ranker.service";
import { StoreVersioningService } from "./store-versioning.service";

export interface UnionStoreConfig {
  name: string;
  version?: string;  // Default: active version
  weight?: number;   // Score weight, default 1.0
}

export type MergeStrategy = "interleave" | "concatenate" | "rrf";

export interface UnionQueryConfig {
  stores: UnionStoreConfig[];
  mergeStrategy: MergeStrategy;
}

export interface UnionSearchResult extends HybridSearchResult {
  source_store: string;
  source_version: string;
}

@Injectable()
export class UnionQueryService {
  private readonly logger = new Logger(UnionQueryService.name);
  private readonly rrfK = 60; // RRF constant

  constructor(private storeVersioning: StoreVersioningService) {
    this.logger.log("UnionQueryService initialized");
  }

  /**
   * Merge results from multiple stores using the specified strategy
   */
  mergeResults(
    resultSets: Array<{
      results: HybridSearchResult[];
      store: UnionStoreConfig;
      version: string;
    }>,
    config: UnionQueryConfig,
  ): UnionSearchResult[] {
    switch (config.mergeStrategy) {
      case "interleave":
        return this.interleaveResults(resultSets);
      case "concatenate":
        return this.concatenateResults(resultSets);
      case "rrf":
        return this.rrfMergeResults(resultSets);
      default:
        return this.rrfMergeResults(resultSets);
    }
  }

  /**
   * Round-robin interleave results from multiple stores
   */
  private interleaveResults(
    resultSets: Array<{
      results: HybridSearchResult[];
      store: UnionStoreConfig;
      version: string;
    }>,
  ): UnionSearchResult[] {
    const merged: UnionSearchResult[] = [];
    const indices = resultSets.map(() => 0);
    const maxResults = 100;

    while (merged.length < maxResults) {
      let added = false;

      for (let i = 0; i < resultSets.length; i++) {
        const { results, store, version } = resultSets[i];

        if (indices[i] < results.length) {
          const result = results[indices[i]];
          const weight = store.weight ?? 1.0;

          merged.push({
            ...result,
            final_score: result.final_score * weight,
            source_store: store.name,
            source_version: version,
          });

          indices[i]++;
          added = true;
        }
      }

      if (!added) break;
    }

    return merged;
  }

  /**
   * Concatenate results from multiple stores, then sort by score
   */
  private concatenateResults(
    resultSets: Array<{
      results: HybridSearchResult[];
      store: UnionStoreConfig;
      version: string;
    }>,
  ): UnionSearchResult[] {
    const merged: UnionSearchResult[] = [];

    for (const { results, store, version } of resultSets) {
      const weight = store.weight ?? 1.0;

      for (const result of results) {
        merged.push({
          ...result,
          final_score: result.final_score * weight,
          source_store: store.name,
          source_version: version,
        });
      }
    }

    // Sort by final score (descending)
    merged.sort((a, b) => b.final_score - a.final_score);

    return merged.slice(0, 100);
  }

  /**
   * Reciprocal Rank Fusion across multiple stores
   * Each store's results contribute to a unified score
   */
  private rrfMergeResults(
    resultSets: Array<{
      results: HybridSearchResult[];
      store: UnionStoreConfig;
      version: string;
    }>,
  ): UnionSearchResult[] {
    // Map: unique key -> { result, rrfScore, sourceInfo }
    const scoreMap = new Map<
      string,
      {
        result: HybridSearchResult;
        rrfScore: number;
        sourceStore: string;
        sourceVersion: string;
      }
    >();

    for (const { results, store, version } of resultSets) {
      const weight = store.weight ?? 1.0;

      for (let rank = 0; rank < results.length; rank++) {
        const result = results[rank];
        // Create unique key combining store and doc_id
        const key = `${store.name}:${result.doc_id}`;
        const rrfScore = weight / (this.rrfK + rank + 1);

        const existing = scoreMap.get(key);
        if (existing) {
          existing.rrfScore += rrfScore;
        } else {
          scoreMap.set(key, {
            result: { ...result },
            rrfScore,
            sourceStore: store.name,
            sourceVersion: version,
          });
        }
      }
    }

    // Convert to array and sort by RRF score
    const merged = Array.from(scoreMap.values())
      .sort((a, b) => b.rrfScore - a.rrfScore)
      .map(({ result, rrfScore, sourceStore, sourceVersion }) => ({
        ...result,
        final_score: rrfScore,
        source_store: sourceStore,
        source_version: sourceVersion,
      }));

    return merged.slice(0, 100);
  }

  /**
   * Validate union query configuration
   */
  validateConfig(config: UnionQueryConfig): void {
    if (!config.stores || config.stores.length === 0) {
      throw new Error("At least one store must be specified for union query");
    }

    for (const store of config.stores) {
      if (!this.storeVersioning.storeExists(store.name)) {
        throw new Error(`Store ${store.name} not found`);
      }

      if (store.version) {
        const version = this.storeVersioning.getVersion(store.name, store.version);
        if (!version) {
          throw new Error(`Version ${store.version} not found in store ${store.name}`);
        }
      }

      if (store.weight !== undefined && (store.weight < 0 || store.weight > 10)) {
        throw new Error(`Invalid weight ${store.weight} for store ${store.name}`);
      }
    }
  }

  /**
   * Get version names for each store in the union config
   */
  getStoreVersionNames(config: UnionQueryConfig): Array<{
    store: UnionStoreConfig;
    milvusCollection: string;
    tantivyIndex: string;
    version: string;
  }> {
    return config.stores.map((store) => {
      const version = store.version ?? 
        this.storeVersioning.getVersionedStore(store.name).activeVersion;
      const names = this.storeVersioning.getVersionNames(store.name, version);

      return {
        store,
        ...names,
        version,
      };
    });
  }
}
