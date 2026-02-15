<script lang="ts">
	import { onMount } from 'svelte';
	import { getHealth, getServerInfo, type HealthResponse, type ServerInfo } from '$lib/api';

	let health: HealthResponse | null = null;
	let serverInfo: ServerInfo | null = null;
	let loading = true;
	let error = '';

	onMount(async () => {
		try {
			[health, serverInfo] = await Promise.all([getHealth(), getServerInfo()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	});
</script>

<h1>Server Home</h1>

{#if loading}
	<p>Loading server state...</p>
{:else if error}
	<p class="error">Failed to load server state: {error}</p>
{:else}
	<div class="status-grid">
		<section>
			<h2>Backend</h2>
			<p class={health?.status === 'ok' ? 'ok' : 'fail'}>{health?.status === 'ok' ? 'ok' : 'fail'}</p>
		</section>

		<section>
			<h2>Identity</h2>
			<p>Name: {serverInfo?.name}</p>
			<p>Fingerprint: {serverInfo?.publicKeyFingerprintEmoji}</p>
		</section>
	</div>
{/if}

<style>
	.status-grid {
		display: grid;
		gap: 16px;
		grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
	}

	section {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
	}

	.ok {
		font-weight: 700;
		color: #15803d;
	}

	.fail {
		font-weight: 700;
		color: #b91c1c;
	}

	.error {
		color: #b91c1c;
	}
</style>
