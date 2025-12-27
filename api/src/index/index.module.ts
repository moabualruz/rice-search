import { Module } from '@nestjs/common';
import { IndexController } from './index.controller';
import { IndexService } from './index.service';
import { ServicesModule } from '../services/services.module';

@Module({
  imports: [ServicesModule],
  controllers: [IndexController],
  providers: [IndexService],
  exports: [IndexService],
})
export class IndexModule {}
