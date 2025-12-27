import { Injectable, NotFoundException } from '@nestjs/common';
import { StoreManagerService } from '../services/store-manager.service';
import { MilvusService } from '../services/milvus.service';
import { TantivyService } from '../services/tantivy.service';

@Injectable()
export class StoresService {
  constructor(
    private storeManager: StoreManagerService,
    private milvus: MilvusService,
    private tantivy: TantivyService,
  ) {}

  async listStores() {
    const stores = this.storeManager.listStores();
    return { stores };
  }

  async createStore(name: string, description?: string) {
    const store = this.storeManager.createStore(name, description || '');

    // Initialize indexes
    await this.milvus.createCollection(name);

    return {
      name: store.name,
      description: store.description,
      created_at: store.created_at,
    };
  }

  async getStore(name: string) {
    const store = this.storeManager.getStore(name);
    if (!store) {
      throw new NotFoundException(`Store '${name}' not found`);
    }

    const milvusStats = await this.milvus.getCollectionStats(name);
    const tantivyStats = await this.tantivy.stats(name);

    return {
      name: store.name,
      description: store.description,
      created_at: store.created_at,
      updated_at: store.updated_at,
      doc_count: Math.max(milvusStats.count, tantivyStats.num_docs),
    };
  }

  async getStoreStats(name: string) {
    const store = this.storeManager.getStore(name);
    if (!store) {
      throw new NotFoundException(`Store '${name}' not found`);
    }

    const milvusStats = await this.milvus.getCollectionStats(name);
    const tantivyStats = await this.tantivy.stats(name);

    return {
      store: name,
      sparse_index: {
        doc_count: tantivyStats.num_docs,
        segment_count: tantivyStats.num_segments,
      },
      dense_index: {
        doc_count: milvusStats.count,
        exists: milvusStats.exists,
      },
      last_updated: store.updated_at,
    };
  }

  async deleteStore(name: string) {
    if (name === 'default') {
      throw new Error('Cannot delete default store');
    }

    const deleted = this.storeManager.deleteStore(name);
    if (!deleted) {
      throw new NotFoundException(`Store '${name}' not found`);
    }

    // Clean up indexes
    await this.milvus.dropCollection(name);
    await this.tantivy.delete(name, { path: '' });

    return { deleted: true, store: name };
  }
}
