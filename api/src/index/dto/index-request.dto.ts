import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import {
  IsString,
  IsOptional,
  IsArray,
  ValidateNested,
  IsBoolean,
  MaxLength,
} from 'class-validator';
import { Type } from 'class-transformer';

export class FileToIndex {
  @ApiProperty({ description: 'Relative path to the file' })
  @IsString()
  @MaxLength(4096)
  path: string;

  @ApiProperty({ description: 'File content' })
  @IsString()
  content: string;
}

export class IndexFilesRequestDto {
  @ApiProperty({ description: 'Files to index', type: [FileToIndex] })
  @IsArray()
  @ValidateNested({ each: true })
  @Type(() => FileToIndex)
  files: FileToIndex[];

  @ApiPropertyOptional({ description: 'Force re-index even if unchanged' })
  @IsOptional()
  @IsBoolean()
  force?: boolean;

  @ApiPropertyOptional({
    description: 'Deprecated: Always async now. Kept for API compatibility.',
    default: true,
    deprecated: true,
  })
  @IsOptional()
  @IsBoolean()
  async?: boolean;
}

export class DeleteFilesRequestDto {
  @ApiPropertyOptional({ description: 'Specific file paths to delete' })
  @IsOptional()
  @IsArray()
  @IsString({ each: true })
  paths?: string[];

  @ApiPropertyOptional({ description: 'Path prefix to delete (e.g., "src/")' })
  @IsOptional()
  @IsString()
  path_prefix?: string;
}

export class AsyncIndexResponseDto {
  @ApiProperty({ description: 'Job ID for tracking' })
  job_id: string;

  @ApiProperty({ description: 'Status: accepted, processing, completed, failed' })
  status: 'accepted' | 'processing' | 'completed' | 'failed';

  @ApiProperty({ description: 'Number of files accepted' })
  files_accepted: number;

  @ApiProperty({ description: 'Number of chunks queued for embedding' })
  chunks_queued: number;

  @ApiProperty({ description: 'Position in queue' })
  queue_position: number;

  @ApiPropertyOptional({ description: 'Number of unchanged files skipped' })
  skipped_unchanged?: number;

  @ApiPropertyOptional({ description: 'Sparse indexing errors' })
  errors?: string[];
}

export class DeleteResponseDto {
  @ApiProperty({ description: 'Number of chunks deleted from sparse index' })
  sparse_deleted: number;

  @ApiProperty({ description: 'Number of chunks deleted from dense index' })
  dense_deleted: number;

  @ApiProperty({ description: 'Processing time in milliseconds' })
  time_ms: number;
}

export class SyncRequestDto {
  @ApiProperty({
    description: 'List of file paths that currently exist',
    type: [String],
  })
  @IsArray()
  @IsString({ each: true })
  current_paths: string[];
}

export class SyncResponseDto {
  @ApiProperty({ description: 'Number of deleted files removed from index' })
  deleted: number;
}

export class StatsResponseDto {
  @ApiProperty({ description: 'Number of tracked files' })
  tracked_files: number;

  @ApiProperty({ description: 'Total size of tracked files in bytes' })
  total_size: number;

  @ApiProperty({ description: 'Last update timestamp' })
  last_updated: string;
}

// File listing with pagination
export class TrackedFileDto {
  @ApiProperty({ description: 'File path' })
  path: string;

  @ApiProperty({ description: 'File size in bytes' })
  size: number;

  @ApiProperty({ description: 'Content hash' })
  hash: string;

  @ApiProperty({ description: 'When file was indexed' })
  indexed_at: string;

  @ApiProperty({ description: 'Number of chunks' })
  chunk_count: number;

  @ApiPropertyOptional({ description: 'Detected language' })
  language?: string;
}

export class ListFilesResponseDto {
  @ApiProperty({ description: 'Files in current page', type: [TrackedFileDto] })
  files: TrackedFileDto[];

  @ApiProperty({ description: 'Total number of files' })
  total: number;

  @ApiProperty({ description: 'Current page (1-indexed)' })
  page: number;

  @ApiProperty({ description: 'Page size' })
  page_size: number;

  @ApiProperty({ description: 'Total pages' })
  total_pages: number;
}
