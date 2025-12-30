import { test, expect } from "@playwright/test";
import { exec } from "child_process";
import { promisify } from "util";
import path from "path";

const execAsync = promisify(exec);
const binaryName =
  process.platform === "win32" ? "rice-search.exe" : "rice-search";
const binaryPath = path.join(process.cwd(), "..", "..", "build", binaryName); // Adjust based on build location relative to test/e2e

test.describe("CLI Client", () => {
  test("should output version", async () => {
    try {
      const { stdout } = await execAsync(`${binaryPath} version`);
      expect(stdout).toContain("Rice Search");
    } catch (e) {
      // If binary not found, fail with clear message
      console.error("Binary not found at:", binaryPath);
      throw e;
    }
  });

  test("should show help", async () => {
    const { stdout } = await execAsync(`${binaryPath} --help`);
    expect(stdout).toContain("Usage:");
    expect(stdout).toContain("Available Commands:");
  });

  // Add more commands: index, search, etc.
});
