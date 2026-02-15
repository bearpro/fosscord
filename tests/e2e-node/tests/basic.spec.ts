import { describe, expect, it } from 'vitest';

const apiBaseUrl = (process.env.E2E_API_BASE_URL || process.env.API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');

describe('API smoke', () => {
  it('GET /health returns 200 and {status:"ok"}', async () => {
    const response = await fetch(`${apiBaseUrl}/health`);
    const bodyText = await response.text();

    if (response.status !== 200) {
      throw new Error(`expected 200 from ${apiBaseUrl}/health, got ${response.status}, body=${bodyText}`);
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(bodyText);
    } catch (error) {
      throw new Error(`failed to parse /health JSON: ${String(error)} body=${bodyText}`);
    }

    expect(parsed).toEqual({ status: 'ok' });
  });
});
