import { test, expect } from "@playwright/test";
import { exec } from "child_process";
import { promisify } from "util";
import path from "path";
import fs from "fs";
import os from "os";

const execAsync = promisify(exec);
const binaryName =
  process.platform === "win32" ? "rice-search.exe" : "rice-search";
const binaryPath = path.resolve(
  __dirname,
  "..",
  "..",
  "..",
  "..",
  "build",
  binaryName
);

test.describe("CLI Watch Mode", () => {
  const testDir = path.join(
    os.tmpdir(),
    "rice-search-watch-test-" + Date.now()
  );
  const storeName = "watch-test-store-" + Date.now();
  const serverFlag = "-S localhost:50052";

  test.beforeAll(async () => {
    // Create test directory
    if (!fs.existsSync(testDir)) {
      fs.mkdirSync(testDir, { recursive: true });
    }

    // Create a store for testing
    try {
      await execAsync(
        `"${binaryPath}" stores create ${storeName} ${serverFlag}`
      );
    } catch (e) {
      console.log("Store create output:", e);
    }
  });

  test.afterAll(async () => {
    // cleanup test dir
    try {
      fs.rmSync(testDir, { recursive: true, force: true });
    } catch (e) {}

    // cleanup store
    try {
      await execAsync(
        `"${binaryPath}" stores delete ${storeName} --force ${serverFlag}`
      );
    } catch (e) {}

    // Stop any leftover watch daemons?
    try {
      await execAsync(`"${binaryPath}" watch stop --all ${serverFlag}`);
    } catch (e) {}
  });

  test("should watch and sync changes", async () => {
    test.slow(); // Increase timeout

    // 1. Create a file
    const file1 = path.join(testDir, "test1.txt");
    fs.writeFileSync(file1, "initial content");

    // 2. Start watcher as daemon
    console.log("Starting watcher daemon...");
    try {
      const { stdout, stderr } = await execAsync(
        `"${binaryPath}" watch "${testDir}" -s ${storeName} --daemon ${serverFlag}`
      );
      console.log("Watch start stdout:", stdout);
      console.log("Watch start stderr:", stderr);
      expect(stdout).toContain("Watcher started");
    } catch (e: any) {
      console.error("Failed to start watcher:", e.message, e.stdout, e.stderr);
      throw e;
    }

    // 3. Verify watcher is listed
    console.log("Listing watchers...");
    const listOutput = await execAsync(
      `"${binaryPath}" watch list ${serverFlag}`
    );
    console.log("List output:", listOutput.stdout);

    // If list output is empty, wait a bit and retry (it takes time to write state file)
    if (!listOutput.stdout.includes(storeName)) {
      console.log("Watcher not listed yet, waiting...");
      await new Promise((r) => setTimeout(r, 1000));
      const listRetry = await execAsync(
        `"${binaryPath}" watch list ${serverFlag}`
      );
      console.log("List retry output:", listRetry.stdout);
      expect(listRetry.stdout).toContain(storeName);
    }

    // 4. Wait for initial sync
    console.log("Waiting for sync...");
    await new Promise((r) => setTimeout(r, 5000));

    // 5. Verify file indexed
    console.log("Verifying index...");
    try {
      let searchOut = await execAsync(
        `"${binaryPath}" search "initial" -s ${storeName} --format json ${serverFlag}`
      );
      console.log("Search output:", searchOut.stdout);
      let result = JSON.parse(searchOut.stdout);
      expect(result.results.length).toBeGreaterThan(0);
      expect(result.results[0].content).toContain("initial content");
    } catch (e: any) {
      console.error("Search failed:", e.message, e.stdout, e.stderr);
      throw e;
    }

    // 6. Modify file
    console.log("Modifying file...");
    fs.writeFileSync(file1, "updated content");

    // 7. Wait for sync
    await new Promise((r) => setTimeout(r, 5000));

    // 8. Verify update
    console.log("Verifying update...");
    try {
      let searchOut = await execAsync(
        `"${binaryPath}" search "updated" -s ${storeName} --format json ${serverFlag}`
      );
      let result = JSON.parse(searchOut.stdout);
      expect(result.results.length).toBeGreaterThan(0);
      expect(result.results[0].content).toContain("updated content");
    } catch (e: any) {
      console.error("Search update failed:", e.message, e.stdout, e.stderr);
      throw e;
    }

    // 9. Stop watcher
    console.log("Stopping watcher...");
    // Use pid from list if needed, but stop --all should work
    await execAsync(`"${binaryPath}" watch stop --all ${serverFlag}`);

    // Verify stopped
    const listAfter = await execAsync(
      `"${binaryPath}" watch list ${serverFlag}`
    );
    expect(listAfter.stdout).toContain("No active watchers");
  });
});
