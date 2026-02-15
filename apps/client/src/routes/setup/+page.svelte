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

<section class="setup-shell">
	<header class="setup-header">
		<h1>Identity Setup</h1>
		<p>Create a local keypair or import an existing one to start connecting to servers.</p>
	</header>

	{#if identity}
		<IdentityCard
			title="Local Identity"
			fingerprint={identity.fingerprint}
			publicKey={identity.publicKey}
		/>
		<section class="panel">
			<p>Identity is ready. You can now connect to servers.</p>
			<button on:click={() => goto('/servers')}>Go to Servers</button>
		</section>
	{:else}
		<section class="panel">
			<h2>Generate</h2>
			<div class="actions">
				<button on:click={handleGenerate} disabled={loading}>
					{loading ? 'Working...' : 'Generate keypair'}
				</button>
			</div>
		</section>

		<section class="panel">
			<h2>Import keypair JSON</h2>
			<label for="import-json">Paste JSON with `publicKey` and `privateKey`.</label>
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
		</section>
	{/if}

	{#if error}
		<p class="error">{error}</p>
	{/if}
</section>

<style>
	.setup-shell {
		max-width: 860px;
		display: grid;
		gap: 14px;
	}

	.setup-header h1 {
		margin: 0;
	}

	.setup-header p {
		margin: 8px 0 0;
		color: #9fb1cf;
	}

	.panel {
		padding: 14px;
		border-radius: 10px;
		border: 1px solid #2f3c58;
		background: #151c2b;
	}

	.panel h2 {
		margin: 0 0 8px;
		font-size: 16px;
	}

	label {
		display: block;
		color: #9fb1cf;
		font-size: 13px;
		margin-bottom: 6px;
	}

	textarea {
		width: 100%;
		padding: 10px;
		font-family: inherit;
		box-sizing: border-box;
		background: #0f1521;
		border: 1px solid #2f3c58;
		border-radius: 8px;
		color: #e7eefc;
		resize: vertical;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #2f63ff;
		color: white;
		cursor: pointer;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.actions {
		display: flex;
		gap: 8px;
		margin-top: 10px;
	}

	.error {
		color: #ff7d7d;
		margin: 0;
	}
</style>
