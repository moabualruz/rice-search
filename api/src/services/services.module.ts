import { Module, Global } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { EmbeddingsService } from './embeddings.service';
import { MilvusService } from './milvus.service';
import { TantivyService } from './tantivy.service';
import { ChunkerService } from './chunker.service';
import { HybridRankerService } from './hybrid-ranker.service';
import { StoreManagerService } from './store-manager.service';
import { GitignoreService } from './gitignore.service';
import { FileTrackerService } from './file-tracker.service';
import { TreeSitterChunkerService } from './treesitter-chunker.service';
import { EmbeddingQueueService } from './embedding-queue.service';

@Global()
@Module({
  imports: [ConfigModule],
  providers: [
    EmbeddingsService,
    MilvusService,
    TantivyService,
    ChunkerService,
    HybridRankerService,
    StoreManagerService,
    GitignoreService,
    FileTrackerService,
    TreeSitterChunkerService,
    EmbeddingQueueService,
  ],
  exports: [
    EmbeddingsService,
    MilvusService,
    TantivyService,
    ChunkerService,
    HybridRankerService,
    StoreManagerService,
    GitignoreService,
    FileTrackerService,
    TreeSitterChunkerService,
    EmbeddingQueueService,
  ],
})
export class ServicesModule {}
