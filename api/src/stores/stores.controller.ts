import { Controller, Get, Post, Delete, Param, Body } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiParam } from '@nestjs/swagger';
import { StoresService } from './stores.service';

@ApiTags('stores')
@Controller('v1/stores')
export class StoresController {
  constructor(private readonly storesService: StoresService) {}

  @Get()
  @ApiOperation({ summary: 'List all stores' })
  @ApiResponse({ status: 200, description: 'List of stores' })
  async listStores() {
    return this.storesService.listStores();
  }

  @Post()
  @ApiOperation({ summary: 'Create a new store' })
  @ApiResponse({ status: 201, description: 'Store created' })
  async createStore(@Body() body: { name: string; description?: string }) {
    return this.storesService.createStore(body.name, body.description);
  }

  @Get(':store')
  @ApiOperation({ summary: 'Get store details' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiResponse({ status: 200, description: 'Store details' })
  async getStore(@Param('store') store: string) {
    return this.storesService.getStore(store);
  }

  @Get(':store/stats')
  @ApiOperation({ summary: 'Get store statistics' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiResponse({ status: 200, description: 'Store statistics' })
  async getStoreStats(@Param('store') store: string) {
    return this.storesService.getStoreStats(store);
  }

  @Delete(':store')
  @ApiOperation({ summary: 'Delete a store' })
  @ApiParam({ name: 'store', description: 'Store name' })
  @ApiResponse({ status: 200, description: 'Store deleted' })
  async deleteStore(@Param('store') store: string) {
    return this.storesService.deleteStore(store);
  }
}
