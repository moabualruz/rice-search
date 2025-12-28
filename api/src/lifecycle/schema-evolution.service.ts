import { Injectable, Logger } from "@nestjs/common";
import { StoreVersioningService, StoreVersionConfig } from "./store-versioning.service";

export type SchemaChangeType = "embedding_model" | "chunking_strategy" | "chunk_size" | "metadata_field";
export type MigrationStrategy = "rebuild" | "dual_write" | "backfill";

export interface SchemaChange {
  type: SchemaChangeType;
  from: string;
  to: string;
  migrationStrategy: MigrationStrategy;
}

export interface MigrationPlan {
  storeName: string;
  sourceVersion: string;
  targetVersion: string;
  change: SchemaChange;
  estimatedTimeMinutes: number;
  requiresReindex: boolean;
  steps: string[];
  warnings: string[];
}

export interface MigrationProgress {
  storeName: string;
  targetVersion: string;
  status: "pending" | "running" | "completed" | "failed";
  progress: number;  // 0-100
  currentStep: string;
  startTime?: Date;
  endTime?: Date;
  error?: string;
}

@Injectable()
export class SchemaEvolutionService {
  private readonly logger = new Logger(SchemaEvolutionService.name);
  private readonly migrations: Map<string, MigrationProgress> = new Map();

  constructor(private storeVersioning: StoreVersioningService) {
    this.logger.log("SchemaEvolutionService initialized");
  }

  /**
   * Plan a schema migration
   */
  planMigration(storeName: string, change: SchemaChange): MigrationPlan {
    const store = this.storeVersioning.getVersionedStore(storeName);
    const activeVersion = store.versions.find((v) => v.status === "active");

    if (!activeVersion) {
      throw new Error(`No active version found for store ${storeName}`);
    }

    const steps: string[] = [];
    const warnings: string[] = [];
    let requiresReindex = false;
    let estimatedTimeMinutes = 0;

    // Analyze the change type
    switch (change.type) {
      case "embedding_model":
        requiresReindex = true;
        steps.push(
          "1. Create new version with updated embedding model",
          "2. Re-embed all documents using new model",
          "3. Index embeddings to new Milvus collection",
          "4. Verify result quality with sample queries",
          "5. Promote new version to active",
          "6. (Optional) Delete old version",
        );
        estimatedTimeMinutes = Math.ceil(activeVersion.stats.chunkCount / 100);
        warnings.push(
          "Embedding model change requires full reindex",
          `Estimated ${activeVersion.stats.chunkCount} chunks to re-embed`,
        );
        break;

      case "chunking_strategy":
        requiresReindex = true;
        steps.push(
          "1. Create new version with updated chunking strategy",
          "2. Re-parse all source files with new chunker",
          "3. Generate embeddings for new chunks",
          "4. Index to both Tantivy and Milvus",
          "5. Verify result quality with sample queries",
          "6. Promote new version to active",
        );
        estimatedTimeMinutes = Math.ceil(activeVersion.stats.docCount / 50);
        warnings.push(
          "Chunking strategy change requires full reindex",
          `Estimated ${activeVersion.stats.docCount} files to re-process`,
        );
        break;

      case "chunk_size":
        requiresReindex = true;
        steps.push(
          "1. Create new version with updated chunk size",
          "2. Re-chunk all documents",
          "3. Generate embeddings for new chunks",
          "4. Index to both Tantivy and Milvus",
          "5. Promote new version to active",
        );
        estimatedTimeMinutes = Math.ceil(activeVersion.stats.docCount / 50);
        break;

      case "metadata_field":
        // Metadata changes may not require full reindex
        if (change.migrationStrategy === "backfill") {
          requiresReindex = false;
          steps.push(
            "1. Create new version with updated schema",
            "2. Copy existing data to new collections",
            "3. Backfill new metadata field",
            "4. Verify data integrity",
            "5. Promote new version to active",
          );
          estimatedTimeMinutes = Math.ceil(activeVersion.stats.chunkCount / 500);
        } else {
          requiresReindex = true;
          steps.push(
            "1. Create new version with updated schema",
            "2. Re-index all documents with new metadata",
            "3. Promote new version to active",
          );
          estimatedTimeMinutes = Math.ceil(activeVersion.stats.docCount / 50);
        }
        break;
    }

    const nextVersionNumber = store.versions.length + 1;

    return {
      storeName,
      sourceVersion: activeVersion.id,
      targetVersion: `v${nextVersionNumber}`,
      change,
      estimatedTimeMinutes,
      requiresReindex,
      steps,
      warnings,
    };
  }

  /**
   * Start a migration (creates new version)
   */
  startMigration(storeName: string, change: SchemaChange): string {
    const plan = this.planMigration(storeName, change);
    const store = this.storeVersioning.getVersionedStore(storeName);
    const activeVersion = store.versions.find((v) => v.status === "active");

    if (!activeVersion) {
      throw new Error(`No active version found for store ${storeName}`);
    }

    // Apply schema change to create new config
    const newConfig = this.applySchemaChange(activeVersion.config, change);

    // Create new version
    const newVersion = this.storeVersioning.createVersion(storeName, newConfig);

    // Track migration progress
    const migrationKey = `${storeName}:${newVersion.id}`;
    this.migrations.set(migrationKey, {
      storeName,
      targetVersion: newVersion.id,
      status: "pending",
      progress: 0,
      currentStep: "Initialized",
      startTime: new Date(),
    });

    this.logger.log(
      `Started migration for ${storeName}: ${plan.sourceVersion} → ${newVersion.id}`
    );

    return newVersion.id;
  }

  /**
   * Apply a schema change to create new config
   */
  private applySchemaChange(
    currentConfig: StoreVersionConfig,
    change: SchemaChange,
  ): StoreVersionConfig {
    const newConfig = { ...currentConfig };

    switch (change.type) {
      case "embedding_model":
        newConfig.embeddingModel = change.to;
        break;
      case "chunking_strategy":
        newConfig.chunkingStrategy = change.to;
        break;
      case "chunk_size":
        newConfig.maxChunkLines = parseInt(change.to, 10);
        break;
      // metadata_field changes don't affect StoreVersionConfig directly
    }

    return newConfig;
  }

  /**
   * Update migration progress
   */
  updateProgress(
    storeName: string,
    versionId: string,
    progress: number,
    currentStep: string,
  ): void {
    const migrationKey = `${storeName}:${versionId}`;
    const migration = this.migrations.get(migrationKey);

    if (migration) {
      migration.progress = progress;
      migration.currentStep = currentStep;

      if (progress >= 100) {
        migration.status = "completed";
        migration.endTime = new Date();
      } else {
        migration.status = "running";
      }
    }
  }

  /**
   * Mark migration as failed
   */
  failMigration(storeName: string, versionId: string, error: string): void {
    const migrationKey = `${storeName}:${versionId}`;
    const migration = this.migrations.get(migrationKey);

    if (migration) {
      migration.status = "failed";
      migration.error = error;
      migration.endTime = new Date();

      this.logger.error(
        `Migration failed for ${storeName}@${versionId}: ${error}`
      );
    }
  }

  /**
   * Get migration progress
   */
  getProgress(storeName: string, versionId: string): MigrationProgress | undefined {
    const migrationKey = `${storeName}:${versionId}`;
    return this.migrations.get(migrationKey);
  }

  /**
   * List all migrations for a store
   */
  listMigrations(storeName: string): MigrationProgress[] {
    const migrations: MigrationProgress[] = [];

    for (const [key, migration] of this.migrations) {
      if (key.startsWith(`${storeName}:`)) {
        migrations.push(migration);
      }
    }

    return migrations;
  }

  /**
   * Validate that a migration is safe to perform
   */
  validateMigration(storeName: string, change: SchemaChange): {
    valid: boolean;
    errors: string[];
  } {
    const errors: string[] = [];

    // Check if store exists
    if (!this.storeVersioning.storeExists(storeName)) {
      errors.push(`Store ${storeName} not found`);
      return { valid: false, errors };
    }

    // Check if there's an active version
    const store = this.storeVersioning.getVersionedStore(storeName);
    const activeVersion = store.versions.find((v) => v.status === "active");
    if (!activeVersion) {
      errors.push(`No active version found for store ${storeName}`);
    }

    // Check for in-progress migrations
    const inProgressMigrations = Array.from(this.migrations.values()).filter(
      (m) => m.storeName === storeName && m.status === "running"
    );
    if (inProgressMigrations.length > 0) {
      errors.push(
        `Migration already in progress for store ${storeName}: ${inProgressMigrations[0].targetVersion}`
      );
    }

    // Validate change type
    const validTypes: SchemaChangeType[] = [
      "embedding_model",
      "chunking_strategy",
      "chunk_size",
      "metadata_field",
    ];
    if (!validTypes.includes(change.type)) {
      errors.push(`Invalid change type: ${change.type}`);
    }

    return {
      valid: errors.length === 0,
      errors,
    };
  }
}
