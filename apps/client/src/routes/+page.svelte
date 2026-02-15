<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { connectAdmin, getServerInfo, type ServerInfo } from '$lib/api';
	import AddServerModal from '$lib/components/AddServerModal.svelte';
	import { createAdminSessionSignature, generateIdentity } from '$lib/crypto';
	import { IS_SINGLE_SERVER_WEB_MODE, SINGLE_SERVER_BASE_URL } from '$lib/runtime';
	import { loadIdentity, saveIdentity, upsertServer } from '$lib/storage';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let serverInfo: ServerInfo | null = null;
	let loadingServerInfo = false;
	let loadingIdentity = false;
	let error = '';

	let addServerOpen = false;
	let connectResult = '';
	let autoLoginInProgress = false;

	onMount(async () => {
		if (!IS_SINGLE_SERVER_WEB_MODE) {
			if (loadIdentity()) {
				void goto('/servers');
				return;
			}
			void goto('/setup');
			return;
		}

		identity = loadIdentity();
		await refreshServerInfo();
	});

	async function refreshServerInfo() {
		loadingServerInfo = true;
		error = '';
		try {
			serverInfo = await getServerInfo(SINGLE_SERVER_BASE_URL);
			await tryAutoAdminConnect(serverInfo);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load server info';
		} finally {
			loadingServerInfo = false;
		}
	}

	async function tryAutoAdminConnect(info: ServerInfo) {
		if (!identity || autoLoginInProgress) {
			return;
		}
		if (!info.adminPublicKeys.includes(identity.publicKey)) {
			return;
		}

		autoLoginInProgress = true;
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminSessionSignature({
				adminPublicKey: identity.publicKey,
				issuedAt,
				serverFingerprint: info.serverFingerprint,
				adminPrivateKeyBase64: identity.privateKey
			});

			const result = await connectAdmin(
				{
					adminPublicKey: identity.publicKey,
					issuedAt,
					signature,
					clientInfo: {
						displayName: 'Web Admin'
					}
				},
				SINGLE_SERVER_BASE_URL
			);

			upsertServer({
				id: result.serverId,
				name: result.serverName,
				baseUrl: SINGLE_SERVER_BASE_URL,
				serverFingerprint: result.serverFingerprint,
				livekitUrl: result.livekitUrl,
				sessionToken: result.sessionToken,
				channels: result.channels,
				lastConnectedAt: new Date().toISOString()
			});

			void goto(`/server/${result.serverId}`);
		} catch {
			// Keep non-blocking UX for admins if auto-login fails.
		} finally {
			autoLoginInProgress = false;
		}
	}

	async function handleGenerateIdentity() {
		loadingIdentity = true;
		error = '';
		try {
			identity = await generateIdentity();
			saveIdentity(identity);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to generate identity';
		} finally {
			loadingIdentity = false;
		}
	}

	function handleConnected(event: CustomEvent<SavedServer>) {
		upsertServer(event.detail);
		addServerOpen = false;
		connectResult = `Connected to ${event.detail.name}`;
		void goto(`/server/${event.detail.id}`);
	}
</script>

{#if !IS_SINGLE_SERVER_WEB_MODE}
	<p>Redirecting...</p>
{:else}
	<h1>{serverInfo?.name ?? 'Single Server Mode'}</h1>

	{#if loadingServerInfo}
		<p>Loading server info...</p>
	{/if}

	{#if error}
		<p class="error">{error}</p>
	{/if}

	{#if serverInfo}
		<p>Server fingerprint: <strong>{serverInfo.serverFingerprint}</strong></p>
		<p>LiveKit URL: <code>{serverInfo.livekitUrl}</code></p>
	{/if}

	{#if !identity}
		<p>Generate a local identity to get your client public key.</p>
		<button on:click={handleGenerateIdentity} disabled={loadingIdentity}>
			{loadingIdentity ? 'Generating...' : 'Generate identity'}
		</button>
	{:else}
		<section class="card">
			<h2>Your Client Public Key</h2>
			<p class="message">You were not invited to this server.</p>
			<code class="pubkey">{identity.publicKey}</code>
			<div class="actions">
				<button on:click={() => (addServerOpen = true)}>Connect via invite link</button>
			</div>
			{#if autoLoginInProgress}
				<p>Logging in as administrator...</p>
			{/if}
			{#if connectResult}
				<p>{connectResult}</p>
			{/if}
		</section>
	{/if}

	{#if identity}
		<AddServerModal
			open={addServerOpen}
			{identity}
			allowedBaseURL={SINGLE_SERVER_BASE_URL}
			on:close={() => (addServerOpen = false)}
			on:connected={handleConnected}
		/>
	{/if}
{/if}

<style>
	h1 {
		margin: 0;
	}

	p {
		color: #9fb1cf;
	}

	.card {
		padding: 16px;
		border-radius: 12px;
		background: #151c2b;
		border: 1px solid #2f3c58;
		margin: 16px 0;
	}

	.message {
		font-weight: 600;
		color: #f2f7ff;
	}

	.pubkey {
		display: block;
		padding: 12px;
		border-radius: 8px;
		background: #0f1521;
		border: 1px solid #2f3c58;
		word-break: break-all;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #2f63ff;
		color: white;
		cursor: pointer;
	}

	.actions {
		margin-top: 12px;
		display: flex;
		gap: 8px;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: #ff7d7d;
	}
</style>
