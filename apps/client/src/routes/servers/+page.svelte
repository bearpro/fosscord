<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import IdentityCard from '$lib/components/IdentityCard.svelte';
	import { loadIdentity, loadServers } from '$lib/storage';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let servers: SavedServer[] = [];

	onMount(() => {
		identity = loadIdentity();
		if (!identity) {
			void goto('/setup');
			return;
		}
		servers = loadServers();
	});
</script>

<h1>Servers</h1>

{#if identity}
	<IdentityCard
		title="Local Identity"
		fingerprint={identity.fingerprint}
		publicKey={identity.publicKey}
	/>

	{#if servers.length === 0}
		<section class="panel">
			<p>No connected servers yet.</p>
			<p>Use the plus button in the left rail to connect via invite link.</p>
		</section>
	{:else}
		<section class="panel">
			<p>Select a server in the left rail.</p>
			<ul>
				{#each servers as server}
					<li>
						<a href={`/server/${server.id}`}>{server.name}</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
{/if}

<style>
	.panel {
		margin-top: 16px;
		padding: 16px;
		border-radius: 12px;
		background: #161c2a;
		border: 1px solid #28334a;
	}

	ul {
		margin: 8px 0 0;
		padding-left: 20px;
	}

	a {
		color: #9fc2ff;
		text-decoration: none;
	}

	a:hover {
		text-decoration: underline;
	}
</style>
