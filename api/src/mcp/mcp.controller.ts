import { Controller, Post, Get, Body, HttpCode, HttpStatus } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { McpService } from './mcp.service';
import { McpRequestDto, McpResponse, McpToolDefinition } from './dto/mcp.dto';

@ApiTags('mcp')
@Controller('mcp')
export class McpController {
  constructor(private readonly mcpService: McpService) {}

  @Get('tools')
  @ApiOperation({ summary: 'List available MCP tools' })
  @ApiResponse({
    status: 200,
    description: 'List of available tools',
  })
  getTools(): McpToolDefinition[] {
    return this.mcpService.getTools();
  }

  @Post()
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Handle MCP JSON-RPC request' })
  @ApiBody({ type: McpRequestDto })
  @ApiResponse({
    status: 200,
    description: 'JSON-RPC response',
  })
  async handleRequest(@Body() request: McpRequestDto): Promise<McpResponse> {
    return this.mcpService.handleRequest(
      request.method,
      request.params,
      request.id,
    );
  }

  @Post('tools/call')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Call an MCP tool directly' })
  @ApiResponse({
    status: 200,
    description: 'Tool execution result',
  })
  async callTool(
    @Body() body: { name: string; arguments: Record<string, unknown> },
  ): Promise<{ content: unknown; isError?: boolean }> {
    return this.mcpService.callTool(body.name, body.arguments);
  }
}
