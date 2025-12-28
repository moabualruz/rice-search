import { Module } from "@nestjs/common";
import { QueryExpansionService } from "./query-expansion.service";
import { SparseEncoderService } from "./sparse-encoder.service";

/**
 * SparseModule provides advanced sparse retrieval capabilities.
 *
 * Components:
 * - QueryExpansionService: Code-aware query expansion (abbreviations, camelCase, synonyms)
 * - SparseEncoderService: Learned sparse vectors (SPLADE/BGE-M3 ready, with stub fallback)
 *
 * Phase 5 of the Search Platform Specification:
 * - Evolve beyond classical BM25 to learned sparse encoders
 * - Improve recall for synonyms and paraphrases
 * - Prepare infrastructure for GPU-accelerated sparse scoring
 */
@Module({
  providers: [QueryExpansionService, SparseEncoderService],
  exports: [QueryExpansionService, SparseEncoderService],
})
export class SparseModule {}
