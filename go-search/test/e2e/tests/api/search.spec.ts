import { test, expect } from '@playwright/test';

test.describe('Search API', () => {
  test('GET /v1/search should return results', async ({ request }) => {
    const response = await request.get('/v1/search', {
      params: {
        q: 'search',
      }
    });
    expect(response.ok()).toBeTruthy();
    const headers = response.headers();
    expect(headers['content-type']).toContain('application/json');
    
    const body = await response.json();
    expect(Array.isArray(body)).toBeTruthy();
    // Start with empty index assumption, but body should be valid array
  });

  test('POST /v1/search should support code filters', async ({ request }) => {
    const response = await request.post('/v1/search', {
        data: {
            query: 'test',
            filters: {
                language: 'go'
            }
        }
    });
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(Array.isArray(body)).toBeTruthy();
  });
});
