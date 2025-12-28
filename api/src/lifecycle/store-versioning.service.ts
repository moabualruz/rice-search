import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import * as fs from "node:fs";
import * as path from "node:path";

export type VersionStatus = "building" | "ready" | "active" | "deprecated";

export interface StoreVersionConfig {
  embeddingModel: string;
  chunkingStrategy: string;
  maxChunkLines: number;
  overlapLines: number;
}

export interface StoreVersionStats {
  docCount: number;
  chunkCount: number;
  lastIndexed: Date;
}

export interface StoreVersion {
  id: string;
  createdAt: Date;
  status: VersionStatus;
  config: StoreVersionConfig;
  stats: StoreVersionStats;
}

export interface VersionedStore {
  name: string;
  description: string;
  activeVersion: string;
  versions: StoreVersion[];
  createdAt: Date;
  updatedAt: Date;
}

const DEFAULT_VERSION_CONFIG: StoreVersionConfig = {
  embeddingModel: "mxbai-embed-large-v1",
  chunkingStrategy: "treesitter",
  maxChunkLines: 100,
  overlapLines: 10,
};

@Injectable()
export class StoreVersioningService {
  private readonly logger = new Logger(StoreVersioningService.name);
  private readonly dataDir: string;
  private readonly versionedStores: Map<string, VersionedStore> = new Map();

  constructor(private configService: ConfigService) {
    this.dataDir = this.configService.get<string>("data.dir") ?? "/data";
    this.loadVersionedStores();
    this.logger.log("StoreVersioningService initialized");
  }

  /**
   * Load all versioned store metadata from disk
   */
  private loadVersionedStores(): void {
    const storesDir = path.join(this.dataDir, "stores");
    try {
      if (!fs.existsSync(storesDir)) {
        fs.mkdirSync(storesDir, { recursive: true });
        return;
      }

      const entries = fs.readdirSync(storesDir, { withFileTypes: true });
      for (const entry of entries) {
        if (entry.isDirectory()) {
          this.loadStoreMetadata(entry.name);
        }
      }

      this.logger.log(`Loaded ${this.versionedStores.size} versioned stores`);
    } catch (error) {
      this.logger.warn(`Failed to load versioned stores: ${error}`);
    }
  }

  /**
   * Load metadata for a single store
   */
  private loadStoreMetadata(storeName: string): void {
    const metadataPath = this.getStoreMetadataPath(storeName);
    try {
      if (fs.existsSync(metadataPath)) {
        const data = fs.readFileSync(metadataPath, "utf-8");
        const store = JSON.parse(data) as VersionedStore;
        // Convert date strings back to Date objects
        store.createdAt = new Date(store.createdAt);
        store.updatedAt = new Date(store.updatedAt);
        store.versions = store.versions.map((v) => ({
          ...v,
          createdAt: new Date(v.createdAt),
          stats: {
            ...v.stats,
            lastIndexed: new Date(v.stats.lastIndexed),
          },
        }));
        this.versionedStores.set(storeName, store);
      }
    } catch (error) {
      this.logger.warn(`Failed to load store ${storeName}: ${error}`);
    }
  }

  /**
   * Save store metadata to disk
   */
  private saveStoreMetadata(store: VersionedStore): void {
    const storeDir = path.join(this.dataDir, "stores", store.name);
    const metadataPath = path.join(storeDir, "metadata.json");

    try {
      fs.mkdirSync(storeDir, { recursive: true });
      fs.writeFileSync(metadataPath, JSON.stringify(store, null, 2));
      this.versionedStores.set(store.name, store);
    } catch (error) {
      this.logger.error(`Failed to save store ${store.name}: ${error}`);
      throw error;
    }
  }

  /**
   * Get path to store metadata file
   */
  private getStoreMetadataPath(storeName: string): string {
    return path.join(this.dataDir, "stores", storeName, "metadata.json");
  }

  /**
   * Get or create a versioned store
   */
  getVersionedStore(storeName: string): VersionedStore {
    let store = this.versionedStores.get(storeName);

    if (!store) {
      // Create new versioned store with initial v1 version
      store = this.initializeStore(storeName);
    }

    return store;
  }

  /**
   * Initialize a new versioned store with v1
   */
  private initializeStore(storeName: string, description = ""): VersionedStore {
    const now = new Date();

    const v1: StoreVersion = {
      id: "v1",
      createdAt: now,
      status: "active",
      config: { ...DEFAULT_VERSION_CONFIG },
      stats: {
        docCount: 0,
        chunkCount: 0,
        lastIndexed: now,
      },
    };

    const store: VersionedStore = {
      name: storeName,
      description,
      activeVersion: "v1",
      versions: [v1],
      createdAt: now,
      updatedAt: now,
    };

    this.saveStoreMetadata(store);
    this.logger.log(`Initialized versioned store: ${storeName} with v1`);

    return store;
  }

  /**
   * Create a new version of a store
   */
  createVersion(
    storeName: string,
    config: Partial<StoreVersionConfig> = {},
  ): StoreVersion {
    const store = this.getVersionedStore(storeName);
    const versionNumber = store.versions.length + 1;
    const versionId = `v${versionNumber}`;

    const newVersion: StoreVersion = {
      id: versionId,
      createdAt: new Date(),
      status: "building",
      config: {
        ...DEFAULT_VERSION_CONFIG,
        ...config,
      },
      stats: {
        docCount: 0,
        chunkCount: 0,
        lastIndexed: new Date(),
      },
    };

    store.versions.push(newVersion);
    store.updatedAt = new Date();
    this.saveStoreMetadata(store);

    this.logger.log(`Created version ${versionId} for store ${storeName}`);
    return newVersion;
  }

  /**
   * Mark a version as ready (indexing complete)
   */
  markVersionReady(storeName: string, versionId: string): void {
    const store = this.getVersionedStore(storeName);
    const version = store.versions.find((v) => v.id === versionId);

    if (!version) {
      throw new Error(`Version ${versionId} not found in store ${storeName}`);
    }

    if (version.status !== "building") {
      throw new Error(`Version ${versionId} is not in building state`);
    }

    version.status = "ready";
    store.updatedAt = new Date();
    this.saveStoreMetadata(store);

    this.logger.log(`Marked version ${versionId} as ready for store ${storeName}`);
  }

  /**
   * Promote a version to active
   */
  promoteVersion(storeName: string, versionId: string): void {
    const store = this.getVersionedStore(storeName);
    const version = store.versions.find((v) => v.id === versionId);

    if (!version) {
      throw new Error(`Version ${versionId} not found in store ${storeName}`);
    }

    if (version.status !== "ready") {
      throw new Error(`Version ${versionId} is not ready for promotion`);
    }

    // Deprecate current active version
    const currentActive = store.versions.find((v) => v.status === "active");
    if (currentActive) {
      currentActive.status = "deprecated";
    }

    // Activate new version
    version.status = "active";
    store.activeVersion = versionId;
    store.updatedAt = new Date();
    this.saveStoreMetadata(store);

    this.logger.log(
      `Promoted version ${versionId} to active for store ${storeName}` +
      (currentActive ? ` (deprecated ${currentActive.id})` : "")
    );
  }

  /**
   * Get the active version for a store
   */
  getActiveVersion(storeName: string): StoreVersion | undefined {
    const store = this.versionedStores.get(storeName);
    if (!store) return undefined;

    return store.versions.find((v) => v.status === "active");
  }

  /**
   * Get version names for Milvus collection and Tantivy index
   */
  getVersionNames(storeName: string, versionId?: string): {
    milvusCollection: string;
    tantivyIndex: string;
  } {
    const store = this.getVersionedStore(storeName);
    const version = versionId || store.activeVersion;

    return {
      milvusCollection: `lcs_${storeName}_${version}`,
      tantivyIndex: `${storeName}_${version}`,
    };
  }

  /**
   * Get active version names (convenience method)
   */
  getActiveVersionNames(storeName: string): {
    milvusCollection: string;
    tantivyIndex: string;
  } {
    return this.getVersionNames(storeName);
  }

  /**
   * Update version stats after indexing
   */
  updateVersionStats(
    storeName: string,
    versionId: string,
    stats: Partial<StoreVersionStats>,
  ): void {
    const store = this.getVersionedStore(storeName);
    const version = store.versions.find((v) => v.id === versionId);

    if (!version) {
      throw new Error(`Version ${versionId} not found in store ${storeName}`);
    }

    if (stats.docCount !== undefined) version.stats.docCount = stats.docCount;
    if (stats.chunkCount !== undefined) version.stats.chunkCount = stats.chunkCount;
    version.stats.lastIndexed = new Date();
    store.updatedAt = new Date();

    this.saveStoreMetadata(store);
  }

  /**
   * Delete a version (must not be active)
   */
  deleteVersion(storeName: string, versionId: string): void {
    const store = this.getVersionedStore(storeName);
    const version = store.versions.find((v) => v.id === versionId);

    if (!version) {
      throw new Error(`Version ${versionId} not found in store ${storeName}`);
    }

    if (version.status === "active") {
      throw new Error("Cannot delete active version");
    }

    store.versions = store.versions.filter((v) => v.id !== versionId);
    store.updatedAt = new Date();
    this.saveStoreMetadata(store);

    this.logger.log(`Deleted version ${versionId} from store ${storeName}`);
  }

  /**
   * List all versions for a store
   */
  listVersions(storeName: string): StoreVersion[] {
    const store = this.versionedStores.get(storeName);
    return store?.versions ?? [];
  }

  /**
   * List all versioned stores
   */
  listStores(): string[] {
    return Array.from(this.versionedStores.keys());
  }

  /**
   * Check if a store exists
   */
  storeExists(storeName: string): boolean {
    return this.versionedStores.has(storeName);
  }

  /**
   * Get a specific version
   */
  getVersion(storeName: string, versionId: string): StoreVersion | undefined {
    const store = this.versionedStores.get(storeName);
    return store?.versions.find((v) => v.id === versionId);
  }
}
