import type { IdentityRecord, SavedServer } from '$lib/types';

const IDENTITY_KEY = 'fosscord.identity.v1';
const SERVERS_KEY = 'fosscord.servers.v1';

function safeParseJSON<T>(raw: string | null): T | null {
	if (!raw) {
		return null;
	}

	try {
		return JSON.parse(raw) as T;
	} catch {
		return null;
	}
}

export function loadIdentity(): IdentityRecord | null {
	return safeParseJSON<IdentityRecord>(localStorage.getItem(IDENTITY_KEY));
}

export function saveIdentity(identity: IdentityRecord): void {
	localStorage.setItem(IDENTITY_KEY, JSON.stringify(identity));
}

export function loadServers(): SavedServer[] {
	return safeParseJSON<SavedServer[]>(localStorage.getItem(SERVERS_KEY)) ?? [];
}

export function saveServers(servers: SavedServer[]): void {
	localStorage.setItem(SERVERS_KEY, JSON.stringify(servers));
}

export function upsertServer(server: SavedServer): SavedServer[] {
	const current = loadServers();
	const index = current.findIndex((item) => item.id === server.id);

	if (index >= 0) {
		current[index] = server;
	} else {
		current.push(server);
	}

	saveServers(current);
	return current;
}

export function getServerByID(serverID: string): SavedServer | null {
	return loadServers().find((server) => server.id === serverID) ?? null;
}
