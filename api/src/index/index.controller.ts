import {
  Controller,
  Post,
  Delete,
  Get,
  Param,
  Body,
  Query,
  HttpCode,
  HttpStatus,
} from '@nestjs/common';
import {
  ApiTags,
  ApiOperation,
  ApiResponse,
  ApiParam,
  ApiBody,
} from '@nestjs/swagger';
import { IndexService } from './index.service';
import {
  IndexFilesRequestDto,
  DeleteFilesRequestDto,
  SyncRequestDto,
  IndexResponseDto,
  AsyncIndexResponseDto,
  DeleteResponseDto,
  SyncResponseDto,
  StatsResponseDto,
  ListFilesResponseDto,
} from './dto/index-request.dto';

@ApiTags('index')
@Controller('v1/stores/:store/index')
export class IndexController {
  constructor(private readonly indexService: IndexService) {}

  @Post()
  @HttpCode(HttpStatus.OK)
  @ApiOperation({
    summary: 'Index files into the store',
    description: 'Set async=true to return immediately while embeddings process in background',
  })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiBody({ type: IndexFilesRequestDto })
  @ApiResponse({
    status: 200,
    description: 'Files indexed successfully (sync mode)',
    type: IndexResponseDto,
  })
  @ApiResponse({
    status: 202,
    description: 'Files accepted for processing (async mode)',
    type: AsyncIndexResponseDto,
  })
  @ApiResponse({ status: 400, description: 'Invalid request' })
  async indexFiles(
    @Param('store') store: string,
    @Body() request: IndexFilesRequestDto,
  ): Promise<IndexResponseDto | AsyncIndexResponseDto> {
    return this.indexService.indexFiles(
      store,
      request.files,
      request.force,
      request.async,
    );
  }

  @Delete()
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Delete files from the store index' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiBody({ type: DeleteFilesRequestDto })
  @ApiResponse({
    status: 200,
    description: 'Files deleted successfully',
    type: DeleteResponseDto,
  })
  async deleteFiles(
    @Param('store') store: string,
    @Body() request: DeleteFilesRequestDto,
  ): Promise<DeleteResponseDto> {
    return this.indexService.deleteFiles(
      store,
      request.paths,
      request.path_prefix,
    );
  }

  @Post('reindex')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Clear and rebuild the entire store index' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiBody({ type: IndexFilesRequestDto })
  @ApiResponse({
    status: 200,
    description: 'Store reindexed successfully',
    type: IndexResponseDto,
  })
  async reindex(
    @Param('store') store: string,
    @Body() request: IndexFilesRequestDto,
  ): Promise<IndexResponseDto> {
    return this.indexService.reindex(store, request.files);
  }

  @Post('sync')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({
    summary: 'Sync index with current files - remove deleted files',
  })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiBody({ type: SyncRequestDto })
  @ApiResponse({
    status: 200,
    description: 'Sync completed successfully',
    type: SyncResponseDto,
  })
  async syncDeletedFiles(
    @Param('store') store: string,
    @Body() request: SyncRequestDto,
  ): Promise<SyncResponseDto> {
    return this.indexService.syncDeletedFiles(store, request.current_paths);
  }

  @Get('stats')
  @ApiOperation({ summary: 'Get indexing statistics for the store' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiResponse({
    status: 200,
    description: 'Store statistics',
    type: StatsResponseDto,
  })
  getStats(@Param('store') store: string): StatsResponseDto {
    return this.indexService.getStoreStats(store);
  }

  @Get('files')
  @ApiOperation({ summary: 'List indexed files with pagination and filtering' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiResponse({
    status: 200,
    description: 'Paginated list of indexed files',
    type: ListFilesResponseDto,
  })
  listFiles(
    @Param('store') store: string,
    @Query('page') page?: string,
    @Query('page_size') pageSize?: string,
    @Query('path') pathFilter?: string,
    @Query('language') language?: string,
    @Query('sort_by') sortBy?: 'path' | 'size' | 'indexed_at',
    @Query('sort_order') sortOrder?: 'asc' | 'desc',
  ): ListFilesResponseDto {
    return this.indexService.listFiles(store, {
      page: page ? parseInt(page, 10) : undefined,
      pageSize: pageSize ? parseInt(pageSize, 10) : undefined,
      pathFilter,
      language,
      sortBy,
      sortOrder,
    });
  }
}
