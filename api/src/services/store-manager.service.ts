import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as fs from 'fs';
import * as path from 'path';

export interface StoreInfo {
  name: string;
  description: string;
  created_at: Date;
  updated_at: Date;
  doc_count: number;
  chunk_count: number;
}

interface StoreMetadata {
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

@Injectable()
export class StoreManagerService {
  private readonly logger = new Logger(StoreManagerService.name);
  private readonly dataDir: string;
  private readonly storesFile: string;
  private stores: Map<string, StoreMetadata> = new Map();

  constructor(private configService: ConfigService) {
    this.dataDir = this.configService.get<string>('data.dir')!;
    this.storesFile = path.join(this.dataDir, 'stores.json');
    this.loadStores();
  }

  private loadStores(): void {
    try {
      if (fs.existsSync(this.storesFile)) {
        const data = fs.readFileSync(this.storesFile, 'utf-8');
        const storesArray: StoreMetadata[] = JSON.parse(data);
        this.stores = new Map(storesArray.map((s) => [s.name, s]));
        this.logger.log(`Loaded ${this.stores.size} stores from disk`);
      } else {
        // Create default store
        this.createStore('default', 'Default code search store');
      }
    } catch (error) {
      this.logger.warn(`Failed to load stores: ${error}`);
      this.stores = new Map();
    }
  }

  private saveStores(): void {
    try {
      fs.mkdirSync(this.dataDir, { recursive: true });
      const storesArray = Array.from(this.stores.values());
      fs.writeFileSync(this.storesFile, JSON.stringify(storesArray, null, 2));
    } catch (error) {
      this.logger.error(`Failed to save stores: ${error}`);
    }
  }

  listStores(): string[] {
    return Array.from(this.stores.keys());
  }

  getStore(name: string): StoreMetadata | undefined {
    return this.stores.get(name);
  }

  storeExists(name: string): boolean {
    return this.stores.has(name);
  }

  createStore(name: string, description = ''): StoreMetadata {
    if (this.stores.has(name)) {
      return this.stores.get(name)!;
    }

    const now = new Date().toISOString();
    const metadata: StoreMetadata = {
      name,
      description,
      created_at: now,
      updated_at: now,
    };

    this.stores.set(name, metadata);
    this.saveStores();
    this.logger.log(`Created store: ${name}`);

    return metadata;
  }

  updateStore(name: string, description?: string): StoreMetadata | undefined {
    const store = this.stores.get(name);
    if (!store) {
      return undefined;
    }

    if (description !== undefined) {
      store.description = description;
    }
    store.updated_at = new Date().toISOString();

    this.stores.set(name, store);
    this.saveStores();

    return store;
  }

  touchStore(name: string): void {
    const store = this.stores.get(name);
    if (store) {
      store.updated_at = new Date().toISOString();
      this.stores.set(name, store);
      this.saveStores();
    }
  }

  deleteStore(name: string): boolean {
    if (name === 'default') {
      this.logger.warn('Cannot delete default store');
      return false;
    }

    const deleted = this.stores.delete(name);
    if (deleted) {
      this.saveStores();
      this.logger.log(`Deleted store: ${name}`);
    }
    return deleted;
  }

  /**
   * Get store info with counts from indexes
   */
  async getStoreInfo(
    name: string,
    sparseCount: number,
    denseCount: number,
  ): Promise<StoreInfo | undefined> {
    const metadata = this.stores.get(name);
    if (!metadata) {
      return undefined;
    }

    return {
      name: metadata.name,
      description: metadata.description,
      created_at: new Date(metadata.created_at),
      updated_at: new Date(metadata.updated_at),
      doc_count: Math.max(sparseCount, denseCount),
      chunk_count: Math.max(sparseCount, denseCount),
    };
  }

  /**
   * Ensure store exists, create if it doesn't
   */
  ensureStore(name: string): StoreMetadata {
    if (!this.stores.has(name)) {
      return this.createStore(name);
    }
    return this.stores.get(name)!;
  }
}
