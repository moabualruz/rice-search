import { test, expect } from "@playwright/test";

test.describe("Observability & Evaluation API", () => {
  test("should export query logs in JSONL format", async ({ request }) => {
    // 1. Generate some queries
    const queries = ["obs-test-1", "obs-test-2"];
    for (const q of queries) {
      await request.post("/v1/search", {
        data: { query: q, store: "default" },
      });
    }

    // Allow async logging time to flush
    await new Promise((r) => setTimeout(r, 1000));

    // 2. Export logs
    const response = await request.get("/v1/observability/export", {
      params: { format: "jsonl", days: "1" },
    });
    expect(response.ok()).toBeTruthy();
    expect(response.headers()["content-type"]).toContain(
      "application/x-jsonlines"
    );

    const text = await response.text();
    const lines = text.trim().split("\n");
    expect(lines.length).toBeGreaterThanOrEqual(2);

    // Check if logs contain our queries
    const found1 = lines.some((l) => l.includes("obs-test-1"));
    const found2 = lines.some((l) => l.includes("obs-test-2"));
    expect(found1).toBeTruthy();
    expect(found2).toBeTruthy();
  });

  test("should export query logs in CSV format", async ({ request }) => {
    // 1. Generate query
    await request.post("/v1/search", {
      data: { query: "csv-test", store: "default" },
    });

    // 2. Export CSV
    const response = await request.get("/v1/observability/export", {
      params: { format: "csv", days: "1" },
    });
    expect(response.ok()).toBeTruthy();
    expect(response.headers()["content-type"]).toContain("text/csv");

    const text = await response.text();
    const lines = text.trim().split("\n");
    expect(lines[0]).toContain("Timestamp,Store,Query"); // Header
    expect(text).toContain("csv-test");
  });

  test("should evaluate queries", async ({ request }) => {
    // Index dummy data to ensure store exists/works
    await request.post("/v1/stores/default/index", {
      data: {
        files: [{ path: "eval.go", content: "func main() {}", language: "go" }],
      },
    });
    await new Promise((r) => setTimeout(r, 1000));

    const response = await request.post("/v1/evaluation/evaluate", {
      data: {
        queries: [
          { id: "q1", query: "auth" },
          { id: "q2", query: "middleware" },
        ],
        store: "default",
        ks: [1, 5],
      },
    });

    if (!response.ok()) {
      console.log("Evaluate failed:", response.status(), await response.text());
    }
    expect(response.ok()).toBeTruthy();
    const body = await response.json();

    expect(body.results).toHaveLength(2);
    expect(body.summary).toBeDefined();
    expect(body.summary.query_count).toBe(2);
    expect(body.results[0].query).toBe("auth");
    expect(body.results[0].ndcg).toBeDefined();
  });
});
