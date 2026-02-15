const rawMode = (import.meta.env.VITE_CLIENT_MODE as string | undefined) ?? 'desktop';

export const CLIENT_MODE = rawMode;
export const IS_SINGLE_SERVER_WEB_MODE = CLIENT_MODE === 'single-server-web';

const rawSingleServerBaseURL =
	(import.meta.env.VITE_SINGLE_SERVER_BASE_URL as string | undefined) ??
	(import.meta.env.VITE_API_BASE_URL as string | undefined) ??
	'http://localhost:8080';

export const SINGLE_SERVER_BASE_URL = rawSingleServerBaseURL.replace(/\/$/, '');
