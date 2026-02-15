import type { Channel } from '$lib/types';

export type HealthResponse = {
	status: 'ok';
};

export type ServerInfo = {
	serverId: string;
	name: string;
	publicKeyFingerprintEmoji: string;
	serverFingerprint: string;
	serverPublicKey: string;
	livekitUrl: string;
	adminPublicKeys: string[];
};

export type ConnectBeginResponse = {
	serverPublicKey: string;
	serverFingerprint: string;
	challenge: string;
	expiresAt: string;
};

export type ConnectFinishResponse = {
	serverId: string;
	serverName: string;
	serverFingerprint: string;
	livekitUrl: string;
	channels: Channel[];
	sessionToken?: string;
};

export type ChannelsResponse = {
	channels: Channel[];
};

export type ConnectFinishRequest = {
	inviteId: string;
	clientPublicKey: string;
	challenge: string;
	signature: string;
	clientInfo?: {
		displayName?: string;
	};
};

export type CreateInviteByClientRequest = {
	adminPublicKey: string;
	clientPublicKey: string;
	label?: string;
	issuedAt: string;
	signature: string;
};

export type ListInvitesByClientRequest = {
	adminPublicKey: string;
	issuedAt: string;
	signature: string;
};

export type CreateInviteResponse = {
	inviteId: string;
	serverBaseUrl: string;
	serverFingerprint: string;
	inviteLink: string;
};

export type InviteSummary = {
	inviteId: string;
	allowedClientPublicKey: string;
	label: string;
	createdAt: string;
	usedAt?: string;
	status: 'active' | 'used' | string;
};

export type ListInvitesResponse = {
	invites: InviteSummary[];
};

export type APIError = {
	error: string;
	message: string;
};

const DEFAULT_API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

async function requestJSON<T>(input: {
	baseUrl?: string;
	path: string;
	method?: 'GET' | 'POST';
	body?: unknown;
}): Promise<T> {
	const baseUrl = (input.baseUrl ?? DEFAULT_API_BASE_URL).replace(/\/$/, '');
	const response = await fetch(`${baseUrl}${input.path}`, {
		method: input.method ?? 'GET',
		headers: input.body
			? {
					'content-type': 'application/json'
				}
			: undefined,
		body: input.body ? JSON.stringify(input.body) : undefined
	});

	const responseText = await response.text();
	if (!response.ok) {
		let parsedError: APIError | null = null;
		try {
			parsedError = JSON.parse(responseText) as APIError;
		} catch {
			// Ignore parse error and use raw response text.
		}
		const message = parsedError?.message ?? (responseText || `request failed (${response.status})`);
		throw new Error(message);
	}

	return JSON.parse(responseText) as T;
}

export function getHealth(baseUrl?: string): Promise<HealthResponse> {
	return requestJSON<HealthResponse>({ baseUrl, path: '/health' });
}

export function getServerInfo(baseUrl?: string): Promise<ServerInfo> {
	return requestJSON<ServerInfo>({ baseUrl, path: '/api/server-info' });
}

export function getChannels(baseUrl?: string): Promise<ChannelsResponse> {
	return requestJSON<ChannelsResponse>({ baseUrl, path: '/api/channels' });
}

export function connectBegin(inviteId: string, baseUrl?: string): Promise<ConnectBeginResponse> {
	return requestJSON<ConnectBeginResponse>({
		baseUrl,
		path: '/api/connect/begin',
		method: 'POST',
		body: { inviteId }
	});
}

export function connectFinish(
	request: ConnectFinishRequest,
	baseUrl?: string
): Promise<ConnectFinishResponse> {
	return requestJSON<ConnectFinishResponse>({
		baseUrl,
		path: '/api/connect/finish',
		method: 'POST',
		body: request
	});
}

export function createInviteByClient(
	request: CreateInviteByClientRequest,
	baseUrl?: string
): Promise<CreateInviteResponse> {
	return requestJSON<CreateInviteResponse>({
		baseUrl,
		path: '/api/admin/invites/client-signed',
		method: 'POST',
		body: request
	});
}

export function listInvitesByClient(
	request: ListInvitesByClientRequest,
	baseUrl?: string
): Promise<ListInvitesResponse> {
	return requestJSON<ListInvitesResponse>({
		baseUrl,
		path: '/api/admin/invites/list/client-signed',
		method: 'POST',
		body: request
	});
}
