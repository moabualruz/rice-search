import { IsString, IsOptional, IsNumber, IsBoolean, Min, Max, ValidateNested } from 'class-validator';
import { Type } from 'class-transformer';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';

class SearchFiltersDto {
  @ApiPropertyOptional({ description: 'Path prefix filter' })
  @IsOptional()
  @IsString()
  path_prefix?: string;

  @ApiPropertyOptional({ description: 'Language filter', type: [String] })
  @IsOptional()
  @IsString({ each: true })
  languages?: string[];
}

export class SearchRequestDto {
  @ApiProperty({ description: 'Search query' })
  @IsString()
  query: string;

  @ApiPropertyOptional({ description: 'Number of results', default: 20 })
  @IsOptional()
  @IsNumber()
  @Min(1)
  @Max(100)
  top_k?: number = 20;

  @ApiPropertyOptional({ description: 'Search filters' })
  @IsOptional()
  @ValidateNested()
  @Type(() => SearchFiltersDto)
  filters?: SearchFiltersDto;

  @ApiPropertyOptional({ description: 'Sparse weight', default: 0.5 })
  @IsOptional()
  @IsNumber()
  @Min(0)
  @Max(1)
  sparse_weight?: number = 0.5;

  @ApiPropertyOptional({ description: 'Dense weight', default: 0.5 })
  @IsOptional()
  @IsNumber()
  @Min(0)
  @Max(1)
  dense_weight?: number = 0.5;

  @ApiPropertyOptional({ description: 'Group results by file', default: false })
  @IsOptional()
  @IsBoolean()
  group_by_file?: boolean = false;

  @ApiPropertyOptional({ description: 'Include content in results', default: true })
  @IsOptional()
  @IsBoolean()
  include_content?: boolean = true;
}
