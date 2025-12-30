import { test, expect } from "@playwright/test";

test.describe("Stores API", () => {
  test.beforeEach(async ({ request }) => {
    // Cleanup potential test store
    await request.delete("/v1/stores/e2e-test-store");
  });

  test("CRUD operations", async ({ request }) => {
    // Create
    const create = await request.post("/v1/stores", {
      data: {
        name: "e2e-test-store",
        description: "Created by E2E test",
      },
    });
    expect(create.ok()).toBeTruthy();

    // List
    const list = await request.get("/v1/stores");
    expect(list.ok()).toBeTruthy();
    const listBody = await list.json();
    expect(listBody).toHaveProperty("data");
    console.log("Stores Body:", JSON.stringify(listBody, null, 2));
    const stores = listBody.data;
    expect(stores.some((s: any) => s.name === "e2e-test-store")).toBeTruthy();

    // Delete
    const del = await request.delete("/v1/stores/e2e-test-store");
    expect(del.ok()).toBeTruthy();

    // Verify Delete
    const list2 = await request.get("/v1/stores");
    const listBody2 = await list2.json();
    expect(listBody2).toHaveProperty("data");
    const stores2 = listBody2.data;
    expect(stores2.some((s: any) => s.name === "e2e-test-store")).toBeFalsy();
  });
});
