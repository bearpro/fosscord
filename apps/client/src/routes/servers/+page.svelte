<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AddServerModal from '$lib/components/AddServerModal.svelte';
	import IdentityCard from '$lib/components/IdentityCard.svelte';
	import { loadIdentity, loadServers, upsertServer } from '$lib/storage';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let servers: SavedServer[] = [];
	let addServerOpen = false;

	onMount(() => {
		identity = loadIdentity();
		if (!identity) {
			void goto('/setup');
			return;
		}
		servers = loadServers();
	});

	function handleConnected(event: CustomEvent<SavedServer>) {
		servers = upsertServer(event.detail);
		addServerOpen = false;
	}
</script>

<h1>Servers</h1>

{#if identity}
	<IdentityCard
		title="Your Identity"
		fingerprint={identity.fingerprint}
		publicKey={identity.publicKey}
	/>

	<div class="header-row">
		<h2>Known servers</h2>
		<button on:click={() => (addServerOpen = true)}>Add server</button>
	</div>

	{#if servers.length === 0}
		<p>No servers connected yet.</p>
	{:else}
		<div class="server-list">
			{#each servers as server}
				<article class="server-card">
					<h3>{server.name}</h3>
					<p>Fingerprint: {server.serverFingerprint}</p>
					<p>Base URL: {server.baseUrl}</p>
					<a href={`/server/${server.id}`}>Open channels</a>
				</article>
			{/each}
		</div>
	{/if}

	<AddServerModal
		open={addServerOpen}
		{identity}
		on:close={() => (addServerOpen = false)}
		on:connected={handleConnected}
	/>
{/if}

<style>
	.header-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-top: 16px;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #0f4c81;
		color: white;
		cursor: pointer;
	}

	.server-list {
		display: grid;
		gap: 12px;
		grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
	}

	.server-card {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
	}
</style>
