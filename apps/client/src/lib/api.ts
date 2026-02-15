export type HealthResponse = {
	status: 'ok';
};

export type ServerInfo = {
	name: string;
	publicKeyFingerprintEmoji: string;
	livekitUrl: string;
};

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';

async function getJSON<T>(path: string): Promise<T> {
	const response = await fetch(`${API_BASE_URL}${path}`);

	if (!response.ok) {
		throw new Error(`Request failed (${response.status}) for ${path}`);
	}

	return (await response.json()) as T;
}

export function getHealth(): Promise<HealthResponse> {
	return getJSON<HealthResponse>('/health');
}

export function getServerInfo(): Promise<ServerInfo> {
	return getJSON<ServerInfo>('/api/server-info');
}
