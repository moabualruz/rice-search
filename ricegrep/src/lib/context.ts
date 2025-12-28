import {
  type FileSystem,
  type FileSystemOptions,
  NodeFileSystem,
} from "./file.js";
import { LOCAL_API_URL, LocalStore } from "./local-store.js";
import { type Store, TestStore } from "./store.js";
import { isTest } from "./utils.js";

/**
 * Creates a Store instance
 * Rice Search is always local - connects to localhost:8080 by default
 * @param options.silent - If true, suppress the backend URL log (default: false)
 */
export async function createStore(options?: { silent?: boolean }): Promise<Store> {
  // Test mode
  if (isTest) {
    return new TestStore();
  }

  // Rice Search mode - always local
  if (!options?.silent) {
    console.log(`Using Rice Search backend: ${LOCAL_API_URL}`);
  }
  return new LocalStore(LOCAL_API_URL);
}

/**
 * Creates a FileSystem instance
 */
export function createFileSystem(
  options: FileSystemOptions = { ignorePatterns: [] },
): FileSystem {
  return new NodeFileSystem(options);
}
