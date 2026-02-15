import nacl from 'tweetnacl';
import type { IdentityRecord } from '$lib/types';

const textEncoder = new TextEncoder();

const fingerprintEmojis = [
	'😀',
	'😎',
	'🚀',
	'🌈',
	'🔥',
	'🧩',
	'🎯',
	'🎧',
	'🛰️',
	'🛡️',
	'🌊',
	'🍀',
	'🧠',
	'🌙',
	'⚡',
	'🧭',
	'🧱',
	'🪐',
	'🍉',
	'🎲',
	'🎹',
	'📡',
	'🧪',
	'🐙',
	'🦊',
	'🦉',
	'🐳',
	'🪁',
	'🏔️',
	'🌵',
	'🍄',
	'🍓'
];

function toBase64(data: Uint8Array): string {
	let binary = '';
	for (let i = 0; i < data.length; i += 1) {
		binary += String.fromCharCode(data[i]);
	}
	return btoa(binary);
}

function fromBase64(value: string): Uint8Array {
	const binary = atob(value);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i += 1) {
		bytes[i] = binary.charCodeAt(i);
	}
	return bytes;
}

async function sha256Bytes(data: Uint8Array): Promise<Uint8Array> {
	const hashBuffer = await crypto.subtle.digest('SHA-256', data as BufferSource);
	return new Uint8Array(hashBuffer);
}

function concatBytes(parts: Uint8Array[]): Uint8Array {
	const total = parts.reduce((acc, part) => acc + part.length, 0);
	const output = new Uint8Array(total);
	let offset = 0;
	for (const part of parts) {
		output.set(part, offset);
		offset += part.length;
	}
	return output;
}

export async function fingerprintFromPublicKey(publicKeyBase64: string): Promise<string> {
	const publicKey = fromBase64(publicKeyBase64);
	if (publicKey.length !== 32) {
		throw new Error('public key must be 32 bytes (Ed25519)');
	}

	const hash = await sha256Bytes(publicKey);
	const parts = Array.from(hash.slice(0, 4)).map(
		(value) => fingerprintEmojis[value % fingerprintEmojis.length]
	);
	return parts.join('');
}

export async function generateIdentity(): Promise<IdentityRecord> {
	const keypair = nacl.sign.keyPair();
	const publicKey = toBase64(keypair.publicKey);
	const privateKey = toBase64(keypair.secretKey);
	const fingerprint = await fingerprintFromPublicKey(publicKey);

	return {
		publicKey,
		privateKey,
		fingerprint,
		createdAt: new Date().toISOString()
	};
}

export async function importIdentityFromJSON(input: string): Promise<IdentityRecord> {
	let parsed: { publicKey?: string; privateKey?: string };
	try {
		parsed = JSON.parse(input) as { publicKey?: string; privateKey?: string };
	} catch (error) {
		throw new Error(`invalid JSON: ${String(error)}`);
	}

	if (!parsed.publicKey || !parsed.privateKey) {
		throw new Error('identity JSON must include publicKey and privateKey');
	}

	const publicKey = fromBase64(parsed.publicKey);
	const privateKey = fromBase64(parsed.privateKey);

	if (publicKey.length !== 32) {
		throw new Error('publicKey must be 32 bytes base64 (Ed25519)');
	}
	if (privateKey.length !== 64) {
		throw new Error('privateKey must be 64 bytes base64 (Ed25519 secretKey)');
	}

	const derivedPublic = privateKey.slice(32);
	if (!derivedPublic.every((value, index) => value === publicKey[index])) {
		throw new Error('publicKey does not match privateKey');
	}

	return {
		publicKey: parsed.publicKey,
		privateKey: parsed.privateKey,
		fingerprint: await fingerprintFromPublicKey(parsed.publicKey),
		createdAt: new Date().toISOString()
	};
}

export async function createHandshakeSignature(input: {
	challengeBase64: string;
	inviteID: string;
	serverFingerprint: string;
	clientPrivateKeyBase64: string;
}): Promise<string> {
	const challenge = fromBase64(input.challengeBase64);
	const secretKey = fromBase64(input.clientPrivateKeyBase64);

	if (secretKey.length !== 64) {
		throw new Error('client private key must be 64 bytes base64 (Ed25519 secretKey)');
	}

	const inviteIDBytes = textEncoder.encode(input.inviteID);
	const serverFingerprintBytes = textEncoder.encode(input.serverFingerprint);
	const payload = concatBytes([challenge, inviteIDBytes, serverFingerprintBytes]);

	const hash = await sha256Bytes(payload);
	const signature = nacl.sign.detached(hash, secretKey);
	return toBase64(signature);
}

export async function createAdminInviteSignature(input: {
	adminPublicKey: string;
	clientPublicKey: string;
	issuedAt: string;
	adminPrivateKeyBase64: string;
}): Promise<string> {
	const secretKey = fromBase64(input.adminPrivateKeyBase64);
	if (secretKey.length !== 64) {
		throw new Error('admin private key must be 64 bytes base64 (Ed25519 secretKey)');
	}

	const adminPublicKeyBytes = textEncoder.encode(input.adminPublicKey);
	const clientPublicKeyBytes = textEncoder.encode(input.clientPublicKey);
	const issuedAtBytes = textEncoder.encode(input.issuedAt);
	const payload = concatBytes([adminPublicKeyBytes, clientPublicKeyBytes, issuedAtBytes]);

	const hash = await sha256Bytes(payload);
	const signature = nacl.sign.detached(hash, secretKey);
	return toBase64(signature);
}
