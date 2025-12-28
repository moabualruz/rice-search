import { Controller, Post, Get, Body, HttpCode, HttpStatus } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse, ApiBody } from '@nestjs/swagger';
import { McpService } from './mcp.service';
import { McpRequestDto, MCP_PROTOCOL_VERSION } from './dto/mcp.dto';
import type {
  McpResponse,
  McpToolDefinition,
  McpResourceDefinition,
  McpResourceTemplate,
  McpPromptDefinition,
  McpServerCapabilities,
} from './dto/mcp.dto';

@ApiTags('mcp')
@Controller('mcp')
export class McpController {
  constructor(private readonly mcpService: McpService) {}

  // ============================================================================
  // Discovery Endpoints (REST convenience)
  // ============================================================================

  @Get('tools')
  @ApiOperation({ summary: 'List available MCP tools' })
  @ApiResponse({
    status: 200,
    description: 'List of available tools',
  })
  getTools(): McpToolDefinition[] {
    return this.mcpService.getTools();
  }

  @Get('resources')
  @ApiOperation({ summary: 'List available MCP resources' })
  @ApiResponse({
    status: 200,
    description: 'List of available resources',
  })
  async getResources(): Promise<McpResourceDefinition[]> {
    return this.mcpService.getResources();
  }

  @Get('resources/templates')
  @ApiOperation({ summary: 'List MCP resource templates' })
  @ApiResponse({
    status: 200,
    description: 'List of resource templates',
  })
  getResourceTemplates(): McpResourceTemplate[] {
    return this.mcpService.getResourceTemplates();
  }

  @Get('prompts')
  @ApiOperation({ summary: 'List available MCP prompts' })
  @ApiResponse({
    status: 200,
    description: 'List of available prompts',
  })
  getPrompts(): McpPromptDefinition[] {
    return this.mcpService.getPrompts();
  }

  @Get('capabilities')
  @ApiOperation({ summary: 'Get server capabilities' })
  @ApiResponse({
    status: 200,
    description: 'Server capabilities',
  })
  getCapabilities(): { protocolVersion: string; capabilities: McpServerCapabilities } {
    return {
      protocolVersion: MCP_PROTOCOL_VERSION,
      capabilities: this.mcpService.getCapabilities(),
    };
  }

  // ============================================================================
  // JSON-RPC Endpoint (MCP Standard)
  // ============================================================================

  @Post()
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Handle MCP JSON-RPC request (MCP Standard)' })
  @ApiBody({ type: McpRequestDto })
  @ApiResponse({
    status: 200,
    description: 'JSON-RPC response',
  })
  async handleRequest(@Body() request: McpRequestDto): Promise<McpResponse | void> {
    // Check if this is a notification (no id)
    if (request.id === undefined || request.id === null) {
      // Notifications don't get a response
      this.mcpService.handleNotification(request.method, request.params);
      return;
    }

    return this.mcpService.handleRequest(request.method, request.params, request.id);
  }

  // ============================================================================
  // Direct Tool Call (Convenience)
  // ============================================================================

  @Post('tools/call')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Call an MCP tool directly (convenience endpoint)' })
  @ApiResponse({
    status: 200,
    description: 'Tool execution result',
  })
  async callTool(
    @Body() body: { name: string; arguments: Record<string, unknown> },
  ): Promise<{ content: unknown; isError?: boolean }> {
    return this.mcpService.callTool(body.name, body.arguments);
  }

  // ============================================================================
  // Direct Resource Read (Convenience)
  // ============================================================================

  @Post('resources/read')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Read an MCP resource directly (convenience endpoint)' })
  @ApiResponse({
    status: 200,
    description: 'Resource contents',
  })
  async readResource(
    @Body() body: { uri: string },
  ): Promise<{ contents: unknown[] }> {
    const contents = await this.mcpService.readResource(body.uri);
    return { contents };
  }

  // ============================================================================
  // Direct Prompt Get (Convenience)
  // ============================================================================

  @Post('prompts/get')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Get an MCP prompt directly (convenience endpoint)' })
  @ApiResponse({
    status: 200,
    description: 'Prompt result',
  })
  async getPrompt(
    @Body() body: { name: string; arguments?: Record<string, string> },
  ): Promise<unknown> {
    return this.mcpService.getPrompt(body.name, body.arguments || {});
  }
}
