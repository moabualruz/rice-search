import { Injectable, Logger } from '@nestjs/common';
import * as fs from 'fs';
import * as path from 'path';
import { DEFAULT_IGNORE_PATTERNS } from '../config/configuration';

/**
 * GitignoreService - Parses .gitignore files and checks file inclusion/exclusion
 */
@Injectable()
export class GitignoreService {
  private readonly logger = new Logger(GitignoreService.name);

  /**
   * Parse a .gitignore file into pattern rules
   */
  parseGitignore(content: string): string[] {
    const patterns: string[] = [];

    for (const line of content.split('\n')) {
      const trimmed = line.trim();

      // Skip empty lines and comments
      if (!trimmed || trimmed.startsWith('#')) {
        continue;
      }

      patterns.push(trimmed);
    }

    return patterns;
  }

  /**
   * Read and parse .gitignore from a directory
   */
  readGitignore(dir: string): string[] {
    const gitignorePath = path.join(dir, '.gitignore');

    try {
      if (fs.existsSync(gitignorePath)) {
        const content = fs.readFileSync(gitignorePath, 'utf-8');
        return this.parseGitignore(content);
      }
    } catch (error) {
      this.logger.warn(`Failed to read .gitignore: ${error}`);
    }

    return [];
  }

  /**
   * Check if a path matches a pattern
   * Supports basic gitignore patterns:
   * - Simple wildcards: *.js, *.py
   * - Directory patterns: node_modules/, dist/
   * - Negation: !important.js
   */
  matchesPattern(filePath: string, pattern: string): boolean {
    const isNegation = pattern.startsWith('!');
    const cleanPattern = isNegation ? pattern.slice(1) : pattern;

    // Normalize paths
    const normalizedPath = filePath.replace(/\\/g, '/');
    const normalizedPattern = cleanPattern.replace(/\\/g, '/');

    // Directory pattern (ends with /)
    if (normalizedPattern.endsWith('/')) {
      const dirName = normalizedPattern.slice(0, -1);
      const pathParts = normalizedPath.split('/');
      return pathParts.some((part) => part === dirName);
    }

    // Simple file extension pattern (*.ext)
    if (normalizedPattern.startsWith('*.')) {
      const ext = normalizedPattern.slice(1);
      return normalizedPath.endsWith(ext);
    }

    // Double star pattern (**/something)
    if (normalizedPattern.startsWith('**/')) {
      const rest = normalizedPattern.slice(3);
      return normalizedPath.includes('/' + rest) || normalizedPath.endsWith(rest);
    }

    // Simple name match (matches anywhere in path)
    if (!normalizedPattern.includes('/')) {
      const pathParts = normalizedPath.split('/');
      return pathParts.some((part) => {
        if (normalizedPattern.includes('*')) {
          return this.wildcardMatch(part, normalizedPattern);
        }
        return part === normalizedPattern;
      });
    }

    // Path pattern - match from root or anywhere
    if (normalizedPattern.startsWith('/')) {
      return this.wildcardMatch(normalizedPath, normalizedPattern.slice(1));
    }

    return (
      normalizedPath.includes('/' + normalizedPattern) ||
      normalizedPath.endsWith(normalizedPattern) ||
      normalizedPath === normalizedPattern
    );
  }

  /**
   * Simple wildcard matching
   */
  private wildcardMatch(str: string, pattern: string): boolean {
    const escapeRegex = (s: string) =>
      s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regexPattern = escapeRegex(pattern)
      .replace(/\\\*/g, '.*')
      .replace(/\\\?/g, '.');
    const regex = new RegExp(`^${regexPattern}$`);
    return regex.test(str);
  }

  /**
   * Check if a file should be ignored based on patterns
   */
  shouldIgnore(filePath: string, patterns: string[]): boolean {
    let ignored = false;

    // Check default patterns first
    for (const defaultPattern of DEFAULT_IGNORE_PATTERNS) {
      if (this.matchesPattern(filePath, defaultPattern)) {
        ignored = true;
        break;
      }
    }

    // Apply custom patterns (can override with negation)
    for (const pattern of patterns) {
      const isNegation = pattern.startsWith('!');

      if (isNegation) {
        // Negation pattern - unignore if matches
        if (this.matchesPattern(filePath, pattern.slice(1))) {
          ignored = false;
        }
      } else {
        // Regular pattern - ignore if matches
        if (this.matchesPattern(filePath, pattern)) {
          ignored = true;
        }
      }
    }

    return ignored;
  }

  /**
   * Get combined ignore patterns for a repository
   */
  getCombinedPatterns(repoRoot: string): string[] {
    const patterns: string[] = [];

    // Add default patterns
    patterns.push(...DEFAULT_IGNORE_PATTERNS);

    // Add .gitignore patterns
    patterns.push(...this.readGitignore(repoRoot));

    // Check for nested .gitignore files (one level deep)
    try {
      const entries = fs.readdirSync(repoRoot, { withFileTypes: true });
      for (const entry of entries) {
        if (entry.isDirectory() && !entry.name.startsWith('.')) {
          const nestedGitignore = this.readGitignore(
            path.join(repoRoot, entry.name),
          );
          // Prefix nested patterns with directory
          for (const pattern of nestedGitignore) {
            if (!pattern.startsWith('!')) {
              patterns.push(`${entry.name}/${pattern}`);
            }
          }
        }
      }
    } catch (error) {
      // Ignore errors reading directories
    }

    return patterns;
  }
}
