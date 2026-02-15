export type ParsedInvite = {
	baseUrl: string;
	inviteID: string;
	serverFingerprint: string;
};

export function parseInviteLink(rawValue: string): ParsedInvite {
	const value = rawValue.trim();
	if (!value) {
		throw new Error('invite link is empty');
	}

	let parsedURL: URL;
	try {
		parsedURL = new URL(value);
	} catch (error) {
		throw new Error(`invalid invite URL: ${String(error)}`);
	}

	const params = parsedURL.searchParams;
	const inviteID = params.get('inviteId') ?? params.get('inviteID') ?? '';
	const serverFingerprint = params.get('serverFp') ?? params.get('serverFingerprint') ?? '';

	let baseUrl = params.get('baseUrl') ?? params.get('serverBaseUrl') ?? '';
	if (!baseUrl && (parsedURL.protocol === 'http:' || parsedURL.protocol === 'https:')) {
		baseUrl = `${parsedURL.protocol}//${parsedURL.host}`;
	}

	if (!baseUrl) {
		throw new Error('invite is missing baseUrl/serverBaseUrl');
	}
	if (!inviteID) {
		throw new Error('invite is missing inviteId');
	}
	if (!serverFingerprint) {
		throw new Error('invite is missing server fingerprint (serverFp/serverFingerprint)');
	}

	return {
		baseUrl: baseUrl.replace(/\/$/, ''),
		inviteID,
		serverFingerprint
	};
}
