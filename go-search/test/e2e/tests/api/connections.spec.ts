import { test, expect } from "@playwright/test";

test.describe("Connections API", () => {
  test("GET /v1/connections should list active connections", async ({
    request,
  }) => {
    const response = await request.get("/v1/connections");
    // Note: If connection tracking is disabled, this might be 404 or empty.
    // But assuming default config or verifying it exists.
    // Based on code, it should be there.
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body).toHaveProperty("data");
    console.log("Connections Body:", JSON.stringify(body, null, 2));
    expect(Array.isArray(body.data)).toBeTruthy();
  });
});
