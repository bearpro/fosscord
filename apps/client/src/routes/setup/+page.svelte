<script lang="ts">
	import { goto } from '$app/navigation';
	import IdentityCard from '$lib/components/IdentityCard.svelte';
	import { generateIdentity, importIdentityFromJSON } from '$lib/crypto';
	import { loadIdentity, saveIdentity } from '$lib/storage';
	import type { IdentityRecord } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let importInput = '';
	let loading = false;
	let error = '';

	identity = loadIdentity();

	async function handleGenerate() {
		loading = true;
		error = '';
		try {
			identity = await generateIdentity();
			saveIdentity(identity);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function handleImport() {
		loading = true;
		error = '';
		try {
			identity = await importIdentityFromJSON(importInput);
			saveIdentity(identity);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}
</script>

<h1>Identity Setup</h1>

{#if identity}
	<IdentityCard
		title="Local Identity"
		fingerprint={identity.fingerprint}
		publicKey={identity.publicKey}
	/>
	<p>Identity is ready. You can now connect to servers.</p>
	<button on:click={() => goto('/servers')}>Go to Servers</button>
{:else}
	<p>Generate a new keypair or import an existing identity JSON.</p>
	<div class="actions">
		<button on:click={handleGenerate} disabled={loading}>
			{loading ? 'Working...' : 'Generate keypair'}
		</button>
	</div>

	<label for="import-json">Import keypair JSON</label>
	<textarea
		id="import-json"
		bind:value={importInput}
		rows="8"
		placeholder="Paste JSON with publicKey and privateKey"
	></textarea>
	<div class="actions">
		<button on:click={handleImport} disabled={loading || !importInput.trim()}>
			{loading ? 'Importing...' : 'Import keypair'}
		</button>
	</div>
{/if}

{#if error}
	<p class="error">{error}</p>
{/if}

<style>
	textarea {
		width: 100%;
		max-width: 760px;
		padding: 8px;
		font-family: inherit;
		box-sizing: border-box;
	}

	button {
		margin-top: 12px;
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #0f4c81;
		color: white;
		cursor: pointer;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.actions {
		margin-bottom: 16px;
	}

	.error {
		color: #b91c1c;
	}
</style>
