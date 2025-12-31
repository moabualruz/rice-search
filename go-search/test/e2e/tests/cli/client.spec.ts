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

  test("should support answer mode (-a)", async () => {
    // 1. Index playwright.config.ts
    const indexCmd = `${binaryPath} index playwright.config.ts -S localhost:50052 -s default`;
    await execAsync(indexCmd);

    // Allow indexing
    await new Promise((r) => setTimeout(r, 1000));

    // 2. Search
    const searchCmd = `${binaryPath} search "defineConfig" -a -S localhost:50052`;
    const { stdout } = await execAsync(searchCmd);

    expect(stdout).toContain("Based on the indexed files");
    expect(stdout).toContain('<cite i="1"/>');
  });

  test("should support verbose mode (-v)", async () => {
    // Reuse index
    const searchCmd = `${binaryPath} search "defineConfig" -v -S localhost:50052`;
    const { stdout } = await execAsync(searchCmd);

    expect(stdout).toContain("Timing Breakdown:");
    expect(stdout).toContain("Embed:");
    expect(stdout).toContain("Retrieve:");
  });
});
