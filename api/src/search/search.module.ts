import { Module } from '@nestjs/common';
import { SearchController } from './search.controller';
import { SearchService } from './search.service';
import { ServicesModule } from '../services/services.module';
import { IntelligenceModule } from '../intelligence/intelligence.module';
import { RankingModule } from '../ranking/ranking.module';
import { PostrankModule } from '../postrank/postrank.module';

@Module({
  imports: [ServicesModule, IntelligenceModule, RankingModule, PostrankModule],
  controllers: [SearchController],
  providers: [SearchService],
  exports: [SearchService],
})
export class SearchModule {}
