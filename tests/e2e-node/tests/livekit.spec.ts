import { createHash, generateKeyPairSync, sign } from 'node:crypto';
import { describe, expect, it } from 'vitest';

const apiBaseUrl = (process.env.E2E_API_BASE_URL || process.env.API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
const livekitUrl = process.env.E2E_LIVEKIT_URL || process.env.LIVEKIT_PUBLIC_URL || 'http://localhost:7880';
const adminToken = process.env.E2E_ADMIN_TOKEN || process.env.ADMIN_TOKEN || 'devadmin';

type ConnectSession = {
  sessionToken: string;
  clientPublicKey: string;
  voiceChannelId: string;
};

function rawPublicKeyBase64FromSpki(spkiDer: Buffer): string {
  if (spkiDer.length < 32) {
    throw new Error('unexpected SPKI DER length');
  }
  return spkiDer.subarray(spkiDer.length - 32).toString('base64');
}

async function requestJSON(input: {
  path: string;
  method?: 'GET' | 'POST';
  body?: unknown;
  headers?: Record<string, string>;
  expectedStatus: number;
}): Promise<unknown> {
  const response = await fetch(`${apiBaseUrl}${input.path}`, {
    method: input.method ?? 'GET',
    headers: {
      ...(input.body
        ? {
            'content-type': 'application/json'
          }
        : {}),
      ...(input.headers ?? {})
    },
    body: input.body ? JSON.stringify(input.body) : undefined
  });

  const bodyText = await response.text();
  if (response.status !== input.expectedStatus) {
    throw new Error(
      `unexpected status for ${input.method ?? 'GET'} ${input.path}: got=${response.status} want=${input.expectedStatus} body=${bodyText}`
    );
  }

  try {
    return JSON.parse(bodyText) as unknown;
  } catch (error) {
    throw new Error(`failed to parse JSON for ${input.path}: ${String(error)} body=${bodyText}`);
  }
}

async function createConnectedSession(): Promise<ConnectSession> {
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  const publicKeyDer = publicKey.export({ format: 'der', type: 'spki' }) as Buffer;
  const clientPublicKey = rawPublicKeyBase64FromSpki(publicKeyDer);

  const invite = (await requestJSON({
    path: '/api/admin/invites',
    method: 'POST',
    expectedStatus: 200,
    headers: {
      authorization: `Bearer ${adminToken}`
    },
    body: {
      clientPublicKey,
      label: 'node-e2e-livekit'
    }
  })) as { inviteId: string };

  const begin = (await requestJSON({
    path: '/api/connect/begin',
    method: 'POST',
    expectedStatus: 200,
    body: {
      inviteId: invite.inviteId
    }
  })) as { challenge: string; serverFingerprint: string };

  const challenge = Buffer.from(begin.challenge, 'base64');
  const signaturePayload = Buffer.concat([
    challenge,
    Buffer.from(invite.inviteId, 'utf8'),
    Buffer.from(begin.serverFingerprint, 'utf8')
  ]);
  const signaturePayloadHash = createHash('sha256').update(signaturePayload).digest();
  const signature = sign(null, signaturePayloadHash, privateKey).toString('base64');

  const finish = (await requestJSON({
    path: '/api/connect/finish',
    method: 'POST',
    expectedStatus: 200,
    body: {
      inviteId: invite.inviteId,
      clientPublicKey,
      challenge: begin.challenge,
      signature,
      clientInfo: {
        displayName: 'Node E2E'
      }
    }
  })) as { sessionToken?: string; channels?: Array<{ id: string; type: string }> };

  if (!finish.sessionToken) {
    throw new Error('connect/finish did not return sessionToken');
  }
  const voiceChannel = (finish.channels ?? []).find((channel) => channel.type === 'voice');
  if (!voiceChannel) {
    throw new Error('server has no voice channels');
  }

  return {
    sessionToken: finish.sessionToken,
    clientPublicKey,
    voiceChannelId: voiceChannel.id
  };
}

describe('LiveKit flow placeholder', () => {
  it('issues voice token for connected session', async (context) => {
    expect(livekitUrl).toBeTruthy();

    const session = await createConnectedSession();
    const response = await fetch(`${apiBaseUrl}/api/livekit/token`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        authorization: `Bearer ${session.sessionToken}`
      },
      body: JSON.stringify({
        channelId: session.voiceChannelId
      })
    });
    const bodyText = await response.text();

    if (response.status === 501) {
      context.skip(`token endpoint is not implemented yet: status=501 body=${bodyText}`);
    }
    if (response.status === 503) {
      context.skip(`LiveKit credentials are not configured: status=503 body=${bodyText}`);
    }
    if (response.status !== 200) {
      throw new Error(`expected 200/501/503 from /api/livekit/token, got ${response.status}, body=${bodyText}`);
    }

    let parsed: { token?: unknown; channelId?: unknown; participantId?: unknown };
    try {
      parsed = JSON.parse(bodyText) as { token?: unknown; channelId?: unknown; participantId?: unknown };
    } catch (error) {
      throw new Error(`failed to parse /api/livekit/token JSON: ${String(error)} body=${bodyText}`);
    }

    expect(typeof parsed.token).toBe('string');
    expect((parsed.token as string).length).toBeGreaterThan(0);
    expect(parsed.channelId).toBe(session.voiceChannelId);
    expect(parsed.participantId).toBe(session.clientPublicKey);
  });

  it.todo('join room and exchange data channel ping/pong between two headless clients');
});
