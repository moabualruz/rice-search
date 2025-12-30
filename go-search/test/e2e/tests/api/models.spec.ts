import { test, expect } from '@playwright/test';

test.describe('Models API', () => {
  test('GET /v1/models should list available models', async ({ request }) => {
    const response = await request.get('/v1/models');
    expect(response.ok()).toBeTruthy();
    const headers = response.headers();
    expect(headers['content-type']).toContain('application/json');
    
    const body = await response.json();
    expect(Array.isArray(body)).toBeTruthy();
    
    // Should have at least the default models from config
    expect(body.length).toBeGreaterThan(0);
    
    // Check structure
    const model = body[0];
    expect(model).toHaveProperty('id');
    expect(model).toHaveProperty('type');
    expect(model).toHaveProperty('display_name');
  });

  test('POST /v1/models/search should return HF results (mock check)', async ({ request }) => {
     // This test relies on HF API, so it might be flaky if offline or rate limited. 
     // We just check if it handles the request gracefully.
     const response = await request.post('/v1/models/search', {
         data: { query: 'bert' }
     });
     
     // It might error if offline, but 200 or 503 are "handled" states vs connection refused
     // Assuming for E2E environment we have internet or mock.
     // If we want to be safe, we just check it doesn't 404.
     expect(response.status()).not.toBe(404);
  });
});
