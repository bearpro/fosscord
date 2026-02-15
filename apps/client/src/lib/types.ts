export type Channel = {
	id: string;
	type: 'text' | 'voice' | string;
	name: string;
};

export type IdentityRecord = {
	publicKey: string;
	privateKey: string;
	fingerprint: string;
	createdAt: string;
};

export type SavedServer = {
	id: string;
	name: string;
	baseUrl: string;
	serverFingerprint: string;
	livekitUrl: string;
	sessionToken?: string;
	channels: Channel[];
	lastConnectedAt: string;
};
