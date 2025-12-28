import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { IsString, IsOptional, IsObject, IsNumber } from 'class-validator';

// MCP Protocol Version
export const MCP_PROTOCOL_VERSION = '2025-11-25';

// ============================================================================
// JSON-RPC Base Types
// ============================================================================

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

export interface McpResponse {
  jsonrpc: '2.0';
  id: number | string | null;
  result?: unknown;
  error?: {
    code: number;
    message: string;
    data?: unknown;
  };
}

export interface McpNotification {
  jsonrpc: '2.0';
  method: string;
  params?: Record<string, unknown>;
}

// ============================================================================
// Tool Definitions (MCP Standard)
// ============================================================================

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
        items?: { type: string };
      }
    >;
    required: string[];
  };
}

// Tool call inputs
export interface CodeSearchInput {
  query: string;
  store?: string;
  top_k?: number;
  path_prefix?: string;
  languages?: string[];
  include_content?: boolean;
}

export interface IndexFilesInput {
  store?: string;
  files: Array<{
    path: string;
    content: string;
  }>;
}

export interface DeleteFilesInput {
  store?: string;
  paths?: string[];
  path_prefix?: string;
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

// ============================================================================
// Resource Definitions (MCP Standard)
// ============================================================================

export interface McpResourceDefinition {
  uri: string;
  name: string;
  description?: string;
  mimeType?: string;
}

export interface McpResourceTemplate {
  uriTemplate: string;
  name: string;
  description?: string;
  mimeType?: string;
}

export interface McpResourceContent {
  uri: string;
  mimeType?: string;
  text?: string;
  blob?: string; // base64 encoded
}

// ============================================================================
// Prompt Definitions (MCP Standard)
// ============================================================================

export interface McpPromptDefinition {
  name: string;
  description?: string;
  arguments?: Array<{
    name: string;
    description?: string;
    required?: boolean;
  }>;
}

export interface McpPromptMessage {
  role: 'user' | 'assistant';
  content: {
    type: 'text' | 'resource';
    text?: string;
    resource?: {
      uri: string;
      mimeType?: string;
      text?: string;
    };
  };
}

export interface McpPromptResult {
  description?: string;
  messages: McpPromptMessage[];
}

// ============================================================================
// Server Capabilities (MCP Standard)
// ============================================================================

export interface McpServerCapabilities {
  tools?: {
    listChanged?: boolean;
  };
  resources?: {
    subscribe?: boolean;
    listChanged?: boolean;
  };
  prompts?: {
    listChanged?: boolean;
  };
  logging?: Record<string, never>;
}

export interface McpServerInfo {
  name: string;
  version: string;
}

export interface McpInitializeResult {
  protocolVersion: string;
  capabilities: McpServerCapabilities;
  serverInfo: McpServerInfo;
}

// ============================================================================
// Error Codes (JSON-RPC Standard)
// ============================================================================

export const McpErrorCodes = {
  PARSE_ERROR: -32700,
  INVALID_REQUEST: -32600,
  METHOD_NOT_FOUND: -32601,
  INVALID_PARAMS: -32602,
  INTERNAL_ERROR: -32603,
} as const;
