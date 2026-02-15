import { describe, expect, it } from 'vitest';

const apiBaseUrl = (process.env.E2E_API_BASE_URL || process.env.API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
const livekitUrl = process.env.E2E_LIVEKIT_URL || process.env.LIVEKIT_PUBLIC_URL || 'http://localhost:7880';

describe('LiveKit flow placeholder', () => {
  it('token endpoint contract scaffold', async (context) => {
    expect(livekitUrl).toBeTruthy();

    const response = await fetch(`${apiBaseUrl}/api/livekit/token`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json'
      },
      body: JSON.stringify({
        room: 'e2e-room',
        identity: 'e2e-client'
      })
    });

    const bodyText = await response.text();

    if (response.status === 501) {
      context.skip(`token endpoint is not implemented yet: status=501 body=${bodyText}`);
    }

    if (response.status !== 200) {
      throw new Error(`expected 200/501 from /api/livekit/token, got ${response.status}, body=${bodyText}`);
    }

    let parsed: { token?: unknown };
    try {
      parsed = JSON.parse(bodyText) as { token?: unknown };
    } catch (error) {
      throw new Error(`failed to parse /api/livekit/token JSON: ${String(error)} body=${bodyText}`);
    }

    expect(typeof parsed.token).toBe('string');
    expect((parsed.token as string).length).toBeGreaterThan(0);
  });

  it.todo('join room and exchange data channel ping/pong between two headless clients');
});
