<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import AddServerModal from '$lib/components/AddServerModal.svelte';
	import { IS_SINGLE_SERVER_WEB_MODE } from '$lib/runtime';
	import { loadIdentity, loadServers, upsertServer } from '$lib/storage';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let servers: SavedServer[] = [];
	let addServerOpen = false;

	function refreshLocalState() {
		if (typeof window === 'undefined') {
			return;
		}
		identity = loadIdentity();
		servers = loadServers();
	}

	function serverPreview(name: string): string {
		const trimmed = name.trim();
		if (!trimmed) {
			return '?';
		}
		return trimmed.slice(0, 1).toUpperCase();
	}

	function shortenPublicKey(value: string): string {
		if (!value) {
			return 'Not configured';
		}
		if (value.length <= 20) {
			return value;
		}
		return `${value.slice(0, 10)}...${value.slice(-8)}`;
	}

	function handleConnected(event: CustomEvent<SavedServer>) {
		servers = upsertServer(event.detail);
		addServerOpen = false;
		void goto(`/server/${event.detail.id}`);
	}

	onMount(() => {
		refreshLocalState();
		window.addEventListener('storage', refreshLocalState);
		return () => {
			window.removeEventListener('storage', refreshLocalState);
		};
	});

	$: if ($page.url.pathname) {
		refreshLocalState();
	}

	$: activeServerID = $page.params.id ?? '';
	$: showServerRail = !IS_SINGLE_SERVER_WEB_MODE && identity !== null;
	$: showAddServer = showServerRail && !$page.url.pathname.startsWith('/setup');
</script>

<svelte:head>
	<title>{IS_SINGLE_SERVER_WEB_MODE ? 'Fosscord Web' : 'Fosscord Desktop'}</title>
</svelte:head>

<div class="app-shell" class:with-rail={showServerRail}>
	{#if showServerRail}
		<aside class="server-rail">
			<a class="server-pill home" href="/servers" aria-label="Servers home">F</a>
			<div class="rail-divider"></div>

			{#each servers as server}
				<a
					class="server-pill"
					class:active={activeServerID === server.id}
					href={`/server/${server.id}`}
					title={server.name}
					aria-label={server.name}
				>
					{serverPreview(server.name)}
				</a>
			{/each}

			{#if showAddServer}
				<button class="server-pill add" on:click={() => (addServerOpen = true)} aria-label="Add server">
					+
				</button>
			{/if}
		</aside>
	{/if}

	<div class="content-column">
		<main>
			<slot />
		</main>

		<footer class="status-bar">
			<span class="status-label">Client Public Key:</span>
			<code title={identity?.publicKey ?? ''}>{shortenPublicKey(identity?.publicKey ?? '')}</code>
		</footer>
	</div>
</div>

{#if identity}
	<AddServerModal
		open={addServerOpen}
		{identity}
		on:close={() => (addServerOpen = false)}
		on:connected={handleConnected}
	/>
{/if}

<style>
	:global(body) {
		margin: 0;
		font-family: 'Iosevka', 'JetBrains Mono', 'Fira Code', monospace;
		background: #101218;
		color: #e2e8f0;
	}

	:global(a) {
		color: inherit;
	}

	.app-shell {
		min-height: 100vh;
		display: grid;
		grid-template-columns: 1fr;
	}

	.app-shell.with-rail {
		grid-template-columns: 76px 1fr;
	}

	.server-rail {
		background: #0b0d12;
		border-right: 1px solid #1e2230;
		padding: 12px 8px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
	}

	.server-pill {
		width: 48px;
		height: 48px;
		border-radius: 50%;
		display: grid;
		place-items: center;
		text-decoration: none;
		font-weight: 700;
		background: #21283a;
		color: #d1d9e9;
		border: 1px solid #2d374f;
		transition: border-color 120ms ease, transform 120ms ease;
	}

	.server-pill:hover {
		border-color: #4f75ff;
		transform: translateY(-1px);
	}

	.server-pill.active {
		background: #2f63ff;
		border-color: #89a6ff;
		color: #ffffff;
	}

	.server-pill.home {
		background: #1e293b;
	}

	.server-pill.add {
		cursor: pointer;
		font-size: 22px;
		background: #152b1f;
		border-color: #274237;
		color: #7ef2ab;
	}

	.rail-divider {
		width: 36px;
		height: 1px;
		background: #2d374f;
	}

	.content-column {
		min-width: 0;
		display: grid;
		grid-template-rows: 1fr auto;
	}

	main {
		padding: 18px;
		min-width: 0;
	}

	.status-bar {
		background: #0d111b;
		border-top: 1px solid #1e2230;
		padding: 8px 14px;
		display: flex;
		gap: 10px;
		align-items: center;
		min-width: 0;
	}

	.status-label {
		color: #8aa1c2;
		white-space: nowrap;
	}

	code {
		color: #d2ddf5;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@media (max-width: 900px) {
		.app-shell.with-rail {
			grid-template-columns: 64px 1fr;
		}

		.server-pill {
			width: 40px;
			height: 40px;
			font-size: 13px;
		}
	}
</style>
