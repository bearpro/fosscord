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

export type AdminConnectRequest = {
	adminPublicKey: string;
	issuedAt: string;
	signature: string;
	clientInfo?: {
		displayName?: string;
	};
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

export type MessageAuthor = {
	displayName: string;
	publicKey: string;
};

export type ChannelMessage = {
	id: string;
	channelId: string;
	author: MessageAuthor;
	contentMarkdown: string;
	createdAt: string;
	updatedAt: string;
};

export type ListMessagesResponse = {
	messages: ChannelMessage[];
};

export type MessageMutationResponse = {
	message: ChannelMessage;
};

export type ChannelStreamEvent = {
	type: 'ready' | 'message.created' | 'message.updated' | string;
	message?: ChannelMessage;
};

export type VoiceParticipant = {
	publicKey: string;
	displayName: string;
	channelId: string;
	joinedAt: string;
	lastSeenAt: string;
	audioStreams: number;
	videoStreams: number;
	cameraEnabled: boolean;
	screenEnabled: boolean;
	screenAudioEnabled: boolean;
};

export type VoiceChannelState = {
	channelId: string;
	participants: VoiceParticipant[];
};

export type LiveKitVoiceTokenResponse = {
	token: string;
	roomName: string;
	channelId: string;
	participantId: string;
};

export type APIError = {
	error: string;
	message: string;
};

const DEFAULT_API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

async function requestJSON<T>(input: {
	baseUrl?: string;
	path: string;
	method?: 'GET' | 'POST' | 'PATCH';
	body?: unknown;
	headers?: Record<string, string>;
}): Promise<T> {
	const baseUrl = (input.baseUrl ?? DEFAULT_API_BASE_URL).replace(/\/$/, '');
	const response = await fetch(`${baseUrl}${input.path}`, {
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

export function connectAdmin(
	request: AdminConnectRequest,
	baseUrl?: string
): Promise<ConnectFinishResponse> {
	return requestJSON<ConnectFinishResponse>({
		baseUrl,
		path: '/api/connect/admin',
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

function authHeaders(sessionToken: string): Record<string, string> {
	return {
		authorization: `Bearer ${sessionToken}`
	};
}

export function getChannelMessages(input: {
	channelId: string;
	sessionToken: string;
	baseUrl?: string;
	limit?: number;
}): Promise<ListMessagesResponse> {
	const limit = input.limit ?? 100;
	return requestJSON<ListMessagesResponse>({
		baseUrl: input.baseUrl,
		path: `/api/channels/${encodeURIComponent(input.channelId)}/messages?limit=${encodeURIComponent(String(limit))}`,
		headers: authHeaders(input.sessionToken)
	});
}

export function createChannelMessage(input: {
	channelId: string;
	sessionToken: string;
	contentMarkdown: string;
	baseUrl?: string;
}): Promise<MessageMutationResponse> {
	return requestJSON<MessageMutationResponse>({
		baseUrl: input.baseUrl,
		path: `/api/channels/${encodeURIComponent(input.channelId)}/messages`,
		method: 'POST',
		headers: authHeaders(input.sessionToken),
		body: {
			contentMarkdown: input.contentMarkdown
		}
	});
}

export function editChannelMessage(input: {
	channelId: string;
	messageId: string;
	sessionToken: string;
	contentMarkdown: string;
	baseUrl?: string;
}): Promise<MessageMutationResponse> {
	return requestJSON<MessageMutationResponse>({
		baseUrl: input.baseUrl,
		path: `/api/channels/${encodeURIComponent(input.channelId)}/messages/${encodeURIComponent(input.messageId)}`,
		method: 'PATCH',
		headers: authHeaders(input.sessionToken),
		body: {
			contentMarkdown: input.contentMarkdown
		}
	});
}

export function createLiveKitVoiceToken(input: {
	channelId: string;
	sessionToken: string;
	baseUrl?: string;
}): Promise<LiveKitVoiceTokenResponse> {
	return requestJSON<LiveKitVoiceTokenResponse>({
		baseUrl: input.baseUrl,
		path: '/api/livekit/token',
		method: 'POST',
		headers: authHeaders(input.sessionToken),
		body: {
			channelId: input.channelId
		}
	});
}

export function touchVoicePresence(input: {
	channelId: string;
	sessionToken: string;
	audioStreams: number;
	videoStreams: number;
	cameraEnabled: boolean;
	screenEnabled: boolean;
	screenAudioEnabled: boolean;
	baseUrl?: string;
}): Promise<{ status: 'ok' }> {
	return requestJSON<{ status: 'ok' }>({
		baseUrl: input.baseUrl,
		path: '/api/livekit/voice/touch',
		method: 'POST',
		headers: authHeaders(input.sessionToken),
		body: {
			channelId: input.channelId,
			audioStreams: input.audioStreams,
			videoStreams: input.videoStreams,
			cameraEnabled: input.cameraEnabled,
			screenEnabled: input.screenEnabled,
			screenAudioEnabled: input.screenAudioEnabled
		}
	});
}

export function leaveVoiceChannel(input: {
	sessionToken: string;
	baseUrl?: string;
}): Promise<{ status: 'ok' }> {
	return requestJSON<{ status: 'ok' }>({
		baseUrl: input.baseUrl,
		path: '/api/livekit/voice/leave',
		method: 'POST',
		headers: authHeaders(input.sessionToken),
		body: {}
	});
}

export function getVoiceChannelState(input: {
	channelId: string;
	sessionToken: string;
	baseUrl?: string;
}): Promise<VoiceChannelState> {
	return requestJSON<VoiceChannelState>({
		baseUrl: input.baseUrl,
		path: `/api/livekit/voice/channels/${encodeURIComponent(input.channelId)}/state`,
		headers: authHeaders(input.sessionToken)
	});
}

export function openChannelStream(input: {
	channelId: string;
	sessionToken: string;
	baseUrl?: string;
}): WebSocket {
	const rawBaseURL = (input.baseUrl ?? DEFAULT_API_BASE_URL).replace(/\/$/, '');
	let resolvedURL: URL;

	if (/^https?:\/\//i.test(rawBaseURL)) {
		resolvedURL = new URL(rawBaseURL);
	} else if (typeof window !== 'undefined') {
		resolvedURL = new URL(rawBaseURL || '/', window.location.origin);
	} else {
		throw new Error('relative websocket URL is not supported outside browser');
	}

	const protocol = resolvedURL.protocol === 'https:' ? 'wss:' : 'ws:';
	const streamURL = new URL(
		`${protocol}//${resolvedURL.host}/api/channels/${encodeURIComponent(input.channelId)}/stream`
	);
	streamURL.searchParams.set('token', input.sessionToken);

	return new WebSocket(streamURL.toString());
}
