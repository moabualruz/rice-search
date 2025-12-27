import { Controller, Post, Param, Body } from "@nestjs/common";
import { ApiTags, ApiOperation, ApiResponse, ApiParam } from "@nestjs/swagger";
import { SearchService } from "./search.service";
import { SearchRequestDto } from "./dto/search-request.dto";

@ApiTags("search")
@Controller("v1/stores")
export class SearchController {
  constructor(private readonly searchService: SearchService) {}

  @Post(":store/search")
  @ApiOperation({ summary: "Hybrid search (sparse + dense)" })
  @ApiParam({ name: "store", description: "Store name" })
  @ApiResponse({ status: 200, description: "Search results" })
  async search(
    @Param("store") store: string,
    @Body() request: SearchRequestDto,
  ) {
    return this.searchService.search(store, request);
  }
}
