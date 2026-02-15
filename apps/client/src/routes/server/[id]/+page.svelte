<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getChannels, getHealth } from '$lib/api';
	import { getServerByID, upsertServer } from '$lib/storage';
	import type { Channel, SavedServer } from '$lib/types';

	let server: SavedServer | null = null;
	let channels: Channel[] = [];
	let backendStatus: 'ok' | 'fail' = 'fail';
	let loading = true;
	let error = '';

	onMount(async () => {
		const serverID = $page.params.id;
		if (!serverID) {
			error = 'Missing server id';
			loading = false;
			return;
		}
		server = getServerByID(serverID);
		if (!server) {
			error = `Unknown server id: ${serverID}`;
			loading = false;
			return;
		}

		try {
			const [health, channelResponse] = await Promise.all([
				getHealth(server.baseUrl),
				getChannels(server.baseUrl)
			]);

			backendStatus = health.status;
			channels = channelResponse.channels;

			upsertServer({
				...server,
				channels,
				lastConnectedAt: new Date().toISOString()
			});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	});
</script>

{#if server}
	<h1>{server.name}</h1>
{/if}

{#if loading}
	<p>Loading server channels...</p>
{:else if error}
	<p class="error">{error}</p>
	<button on:click={() => goto('/servers')}>Back to Servers</button>
{:else}
	<section class="status-card">
		<h2>Status</h2>
		<p>Backend: <strong class={backendStatus === 'ok' ? 'ok' : 'fail'}>{backendStatus}</strong></p>
		<p>Server fingerprint: {server?.serverFingerprint}</p>
	</section>

	<section class="channel-card">
		<h2>Channels</h2>
		{#if channels.length === 0}
			<p>No channels returned by server.</p>
		{:else}
			<ul>
				{#each channels as channel}
					<li>
						<strong>[{channel.type}]</strong> {channel.name}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}

<style>
	.status-card,
	.channel-card {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
		margin-bottom: 12px;
	}

	.ok {
		color: #15803d;
	}

	.fail {
		color: #b91c1c;
	}

	.error {
		color: #b91c1c;
	}
</style>
