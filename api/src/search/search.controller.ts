import { Controller, Post, Param, Body } from "@nestjs/common";
import { ApiTags, ApiOperation, ApiResponse, ApiParam, ApiBody } from "@nestjs/swagger";
import { SearchService } from "./search.service";
import { SearchRequestDto } from "./dto/search-request.dto";
import { SearchResponseDto } from "./dto/search-response.dto";

@ApiTags("search")
@Controller("v1/stores")
export class SearchController {
  constructor(private readonly searchService: SearchService) {}

  @Post(":store/search")
  @ApiOperation({
    summary: "Hybrid search (BM25 + semantic + reranking)",
    description: `
Performs intelligent hybrid search across indexed code.

**Features:**
- **Intent Detection**: Automatically classifies query as navigational, factual, exploratory, or analytical
- **Strategy Selection**: Chooses optimal retrieval strategy based on query type
- **Query Expansion**: Expands abbreviations (auth→authentication) and splits camelCase
- **Hybrid Search**: Combines BM25 keyword search with semantic embeddings
- **Neural Reranking**: Two-pass reranking with early exit optimization
- **Deduplication**: Removes semantically similar results
- **Diversity**: MMR-based result diversification
- **File Grouping**: Optional grouping by file with representative chunks

**Search Options:**
- \`sparse_weight\` / \`dense_weight\`: Control BM25 vs semantic balance
- \`enable_reranking\`: Toggle neural reranking
- \`enable_dedup\`: Toggle semantic deduplication
- \`enable_diversity\`: Toggle MMR diversity
- \`group_by_file\`: Group results by file
- \`enable_expansion\`: Toggle query expansion
    `,
  })
  @ApiParam({ name: "store", description: "Store name (e.g., 'default')" })
  @ApiBody({ type: SearchRequestDto })
  @ApiResponse({
    status: 200,
    description: "Search results with intelligence metadata",
    type: SearchResponseDto,
  })
  @ApiResponse({
    status: 400,
    description: "Invalid request parameters",
  })
  @ApiResponse({
    status: 404,
    description: "Store not found",
  })
  async search(
    @Param("store") store: string,
    @Body() request: SearchRequestDto,
  ) {
    return this.searchService.search(store, request);
  }
}
