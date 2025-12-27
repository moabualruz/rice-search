import { Module, Global } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { EmbeddingsService } from './embeddings.service';
import { InfinityService } from './infinity.service';
import { BgeM3Service } from './bge-m3.service';
import { MilvusService } from './milvus.service';
import { TantivyService } from './tantivy.service';
import { TantivyQueueService } from './tantivy-queue.service';
import { ChunkerService } from './chunker.service';
import { HybridRankerService } from './hybrid-ranker.service';
import { StoreManagerService } from './store-manager.service';
import { GitignoreService } from './gitignore.service';
import { FileTrackerService } from './file-tracker.service';
import { TreeSitterChunkerService } from './treesitter-chunker.service';
import { EmbeddingQueueService } from './embedding-queue.service';
import { RerankerService } from './reranker.service';
import { QueryClassifierService } from './query-classifier.service';

@Global()
@Module({
  imports: [ConfigModule],
  providers: [
    EmbeddingsService,
    InfinityService,
    BgeM3Service,
    MilvusService,
    TantivyService,
    TantivyQueueService, // Dynamic per-store queues (creates Redis connections on-demand)
    ChunkerService,
    HybridRankerService,
    RerankerService,
    StoreManagerService,
    GitignoreService,
    FileTrackerService,
    TreeSitterChunkerService,
    EmbeddingQueueService,
    QueryClassifierService,
  ],
  exports: [
    EmbeddingsService,
    InfinityService,
    BgeM3Service,
    MilvusService,
    TantivyService,
    TantivyQueueService,
    ChunkerService,
    HybridRankerService,
    RerankerService,
    StoreManagerService,
    GitignoreService,
    FileTrackerService,
    TreeSitterChunkerService,
    EmbeddingQueueService,
    QueryClassifierService,
  ],
})
export class ServicesModule {}
