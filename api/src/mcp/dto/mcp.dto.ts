import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { IsString, IsOptional, IsObject, IsNumber } from 'class-validator';

// MCP Tool Definitions
export interface McpToolDefinition {
  name: string;
  description: string;
  inputSchema: {
    type: 'object';
    properties: Record<
      string,
      {
        type: string;
        description: string;
        default?: unknown;
        enum?: string[];
      }
    >;
    required: string[];
  };
}

// MCP JSON-RPC Request
export class McpRequestDto {
  @ApiProperty({ description: 'JSON-RPC version', default: '2.0' })
  @IsString()
  jsonrpc: string = '2.0';

  @ApiProperty({ description: 'Request ID' })
  @IsNumber()
  id: number;

  @ApiProperty({ description: 'Method name' })
  @IsString()
  method: string;

  @ApiPropertyOptional({ description: 'Method parameters' })
  @IsOptional()
  @IsObject()
  params?: Record<string, unknown>;
}

// MCP JSON-RPC Response
export interface McpResponse {
  jsonrpc: '2.0';
  id: number;
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
}

// Tool call input for code_search
export interface CodeSearchInput {
  query: string;
  store?: string;
  top_k?: number;
  path_prefix?: string;
  languages?: string[];
  include_content?: boolean;
}

// Tool call input for index_files
export interface IndexFilesInput {
  store?: string;
  files: Array<{
    path: string;
    content: string;
  }>;
}

// Search result for MCP
export interface McpSearchResult {
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content?: string;
  symbols: string[];
  score: number;
}
