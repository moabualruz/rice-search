import {
  type FileSystem,
  type FileSystemOptions,
  NodeFileSystem,
} from "./file.js";
import { type Git, NodeGit } from "./git.js";
import { LOCAL_API_URL, LocalStore } from "./local-store.js";
import { type Store, TestStore } from "./store.js";
import { isTest } from "./utils.js";

/**
 * Creates a Store instance
 * Rice Search is always local - connects to localhost:8080 by default
 */
export async function createStore(): Promise<Store> {
  // Test mode
  if (isTest) {
    return new TestStore();
  }

  // Rice Search mode - always local
  console.log(`Using Rice Search backend: ${LOCAL_API_URL}`);
  return new LocalStore(LOCAL_API_URL);
}

/**
 * Creates a Git instance
 */
export function createGit(): Git {
  return new NodeGit();
}

/**
 * Creates a FileSystem instance
 */
export function createFileSystem(
  options: FileSystemOptions = { ignorePatterns: [] },
): FileSystem {
  return new NodeFileSystem(createGit(), options);
}
