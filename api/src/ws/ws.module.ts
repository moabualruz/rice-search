import { Module } from '@nestjs/common';
import { WsGateway } from './ws.gateway';
import { FileBufferService } from './file-buffer.service';
import { SearchModule } from '../search/search.module';
import { IndexModule } from '../index/index.module';

@Module({
  imports: [SearchModule, IndexModule],
  providers: [WsGateway, FileBufferService],
  exports: [FileBufferService],
})
export class WsModule {}
