<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { getServerInfo, type ServerInfo } from '$lib/api';
	import AddServerModal from '$lib/components/AddServerModal.svelte';
	import { generateIdentity } from '$lib/crypto';
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
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load server info';
		} finally {
			loadingServerInfo = false;
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
	.card {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
		margin: 16px 0;
	}

	.message {
		font-weight: 600;
	}

	.pubkey {
		display: block;
		padding: 12px;
		border-radius: 8px;
		background: #f1f5f9;
		word-break: break-all;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #0f4c81;
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
		color: #b91c1c;
	}
</style>
