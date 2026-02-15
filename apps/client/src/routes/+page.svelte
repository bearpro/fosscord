<script lang="ts">
	import { onMount } from 'svelte';
	import { getServerInfo, type ServerInfo } from '$lib/api';

	let serverInfo: ServerInfo | null = null;
	let loading = true;
	let error = '';

	onMount(async () => {
		try {
			serverInfo = await getServerInfo();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	});
</script>

<h1>Servers</h1>

{#if loading}
	<p>Loading server list...</p>
{:else if error}
	<p class="error">Failed to load server list: {error}</p>
{:else if serverInfo}
	<article class="server-card">
		<h2>{serverInfo.name}</h2>
		<p>Fingerprint: {serverInfo.publicKeyFingerprintEmoji}</p>
		<p>LiveKit: {serverInfo.livekitUrl}</p>
		<a href="/server">Open Server Home</a>
	</article>
{/if}

<style>
	.server-card {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
	}

	.error {
		color: #b91c1c;
	}
</style>
