import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { spawn } from 'child_process';
import * as path from 'path';

export interface TantivyDocument {
  doc_id: string;
  path: string;
  language: string;
  symbols: string[];
  content: string;
  start_line: number;
  end_line: number;
}

export interface TantivySearchResult {
  doc_id: string;
  path: string;
  language: string;
  symbols: string[];
  content: string;
  start_line: number;
  end_line: number;
  bm25_score: number;
  rank: number;
}

@Injectable()
export class TantivyService {
  private readonly logger = new Logger(TantivyService.name);
  private readonly cliPath: string;
  private readonly indexDir: string;
  private readonly useCargo: boolean;
  private readonly projectDir: string;

  constructor(private configService: ConfigService) {
    this.cliPath = this.configService.get<string>('tantivy.cliPath')!;
    this.indexDir = this.configService.get<string>('data.tantivyDir')!;
    this.useCargo = this.configService.get<boolean>('tantivy.useCargo') || false;
    this.projectDir = this.configService.get<string>('tantivy.projectDir') || '';

    if (this.useCargo) {
      this.logger.log(`Using cargo run from ${this.projectDir}`);
    }
  }

  private async runCommand(
    command: string,
    args: string[],
    stdin?: string,
  ): Promise<string> {
    return new Promise((resolve, reject) => {
      let proc;
      if (this.useCargo) {
        // Use cargo run -- <command> <args>
        proc = spawn('cargo', ['run', '--release', '--', command, ...args], {
          cwd: this.projectDir,
        });
      } else {
        proc = spawn(this.cliPath, [command, ...args]);
      }

      let stdout = '';
      let stderr = '';

      proc.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      proc.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      proc.on('close', (code) => {
        if (code === 0) {
          resolve(stdout);
        } else {
          reject(new Error(`Tantivy CLI exited with code ${code}: ${stderr}`));
        }
      });

      proc.on('error', (err) => {
        reject(err);
      });

      if (stdin) {
        proc.stdin.write(stdin);
        proc.stdin.end();
      }
    });
  }

  private getIndexPath(store: string): string {
    return path.join(this.indexDir, store);
  }

  /**
   * Index documents using Tantivy (direct CLI call).
   * Use TantivyQueueService.index() for queued/serialized access.
   * @param store Store name
   * @param documents Documents to index
   * @returns Indexing result
   */
  async indexDirect(
    store: string,
    documents: TantivyDocument[],
  ): Promise<{ indexed: number; errors: number }> {
    const indexPath = this.getIndexPath(store);

    try {
      // Convert documents to JSON lines
      const jsonLines = documents.map((doc) => JSON.stringify(doc)).join('\n');

      const output = await this.runCommand(
        'index',
        ['--index-path', indexPath, '--store', store],
        jsonLines,
      );

      const result = JSON.parse(output);
      this.logger.log(
        `Indexed ${result.indexed} documents in store ${store}`,
      );
      return result;
    } catch (error) {
      // Don't log here - BullMQ queue handles retry logging
      throw error;
    }
  }

  /**
   * Alias for backward compatibility - prefer TantivyQueueService.index()
   */
  async index(
    store: string,
    documents: TantivyDocument[],
  ): Promise<{ indexed: number; errors: number }> {
    return this.indexDirect(store, documents);
  }

  /**
   * Search documents using Tantivy
   * @param store Store name
   * @param query Search query
   * @param topK Maximum results
   * @param pathPrefix Optional path prefix filter
   * @param language Optional language filter
   * @returns Search results
   */
  async search(
    store: string,
    query: string,
    topK = 200,
    pathPrefix?: string,
    language?: string,
  ): Promise<TantivySearchResult[]> {
    const indexPath = this.getIndexPath(store);

    const args = [
      '--index-path',
      indexPath,
      '--store',
      store,
      '--query',
      query,
      '-k',
      topK.toString(),
    ];

    if (pathPrefix) {
      args.push('--path-prefix', pathPrefix);
    }

    if (language) {
      args.push('--language', language);
    }

    try {
      const output = await this.runCommand('search', args);
      const result = JSON.parse(output);
      return result.results;
    } catch (error) {
      // If index doesn't exist, return empty results
      if (
        error instanceof Error &&
        error.message.includes('Failed to open index')
      ) {
        return [];
      }
      this.logger.error(`Search failed: ${error}`);
      throw error;
    }
  }

  /**
   * Delete documents from Tantivy index (direct CLI call).
   * Use TantivyQueueService.delete() for queued/serialized access.
   * @param store Store name
   * @param options Delete options
   * @returns Number of deleted documents
   */
  async deleteDirect(
    store: string,
    options: { path?: string; docId?: string },
  ): Promise<number> {
    const indexPath = this.getIndexPath(store);

    try {
      const args = ['--index-path', indexPath, '--store', store];

      if (options.path) {
        args.push('--path', options.path);
      }

      if (options.docId) {
        args.push('--doc-id', options.docId);
      }

      const output = await this.runCommand('delete', args);
      const result = JSON.parse(output);
      return result.deleted;
    } catch (error) {
      // Don't log here - BullMQ queue handles retry logging
      throw error;
    }
  }

  /**
   * Alias for backward compatibility - prefer TantivyQueueService.delete()
   */
  async delete(
    store: string,
    options: { path?: string; docId?: string },
  ): Promise<number> {
    return this.deleteDirect(store, options);
  }

  /**
   * Get index statistics
   * @param store Store name
   * @returns Index stats
   */
  async stats(
    store: string,
  ): Promise<{ num_docs: number; num_segments: number }> {
    const indexPath = this.getIndexPath(store);

    try {
      const output = await this.runCommand('stats', [
        '--index-path',
        indexPath,
        '--store',
        store,
      ]);
      return JSON.parse(output);
    } catch (error) {
      // If index doesn't exist, return zeros
      if (
        error instanceof Error &&
        error.message.includes('Failed to open index')
      ) {
        return { num_docs: 0, num_segments: 0 };
      }
      this.logger.error(`Stats failed: ${error}`);
      throw error;
    }
  }
}
