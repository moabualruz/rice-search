import { test, expect } from "@playwright/test";

test.describe("Search API", () => {
  test("should index and search data", async ({ request }) => {
    // 1. Index some data first
    const indexResponse = await request.post("/v1/stores/default/index", {
      data: {
        files: [
          {
            path: "test/hello.go",
            content:
              'package main\n\nfunc main() {\n\tprintln("Hello Search")\n}',
            language: "go",
          },
        ],
      },
    });
    expect(indexResponse.ok()).toBeTruthy();

    // Allow a moment for indexing
    await new Promise((r) => setTimeout(r, 1000));

    // 2. Search for it
    const response = await request.post("/v1/stores/default/search", {
      data: {
        query: "Hello Search",
        top_k: 10,
      },
    });

    expect(response.ok()).toBeTruthy();
    const headers = response.headers();
    expect(headers["content-type"]).toContain("application/json");

    const body = await response.json();
    // Verify response structure matches SearchResponse
    expect(body).toHaveProperty("results");
    expect(Array.isArray(body.results)).toBeTruthy();
    expect(body.results.length).toBeGreaterThan(0);
    expect(body.results[0].path).toBe("test/hello.go");
  });

  test("should support code filters", async ({ request }) => {
    // Search with filter
    const response = await request.post("/v1/stores/default/search", {
      data: {
        query: "Hello",
        filter: {
          language: "go",
        },
      },
    });

    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body.results.length).toBeGreaterThan(0);
    expect(body.results[0].language).toBe("go");
  });
});
