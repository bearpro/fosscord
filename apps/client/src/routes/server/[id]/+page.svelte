<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import {
		createInviteByClient,
		getChannels,
		getHealth,
		getServerInfo,
		listInvitesByClient,
		type InviteSummary
	} from '$lib/api';
	import { createAdminInviteSignature, createAdminListInvitesSignature } from '$lib/crypto';
	import { getServerByID, loadIdentity, upsertServer } from '$lib/storage';
	import type { Channel, IdentityRecord, SavedServer } from '$lib/types';

	let identity: IdentityRecord | null = null;
	let server: SavedServer | null = null;
	let channels: Channel[] = [];
	let backendStatus: 'ok' | 'fail' = 'fail';
	let loading = true;
	let error = '';
	let adminPublicKeys: string[] = [];

	let targetClientPublicKey = '';
	let targetClientLabel = '';
	let creatingInvite = false;
	let createInviteError = '';
	let createdInviteLink = '';

	let loadingInvites = false;
	let invitesError = '';
	let invitesLoaded = false;
	let invites: InviteSummary[] = [];
	let initialized = false;
	let activeServerID = '';

	$: currentView = $page.url.searchParams.get('view') ?? 'channel';
	$: currentChannelID = $page.url.searchParams.get('channel') ?? '';
	$: selectedChannel =
		channels.find((channel) => channel.id === currentChannelID) ?? channels[0] ?? null;
	$: isAdmin = Boolean(identity && adminPublicKeys.includes(identity.publicKey));

	onMount(() => {
		initialized = true;
	});

	async function initializeServer(serverID: string) {
		activeServerID = serverID;
		error = '';
		loading = true;
		invitesLoaded = false;
		invites = [];
		invitesError = '';
		adminPublicKeys = [];

		if (!serverID) {
			error = 'Missing server id';
			loading = false;
			return;
		}

		identity = loadIdentity();
		if (!identity) {
			void goto('/setup');
			return;
		}

		server = getServerByID(serverID);
		if (!server) {
			error = `Unknown server id: ${serverID}`;
			loading = false;
			return;
		}

		await refreshServerState();
	}

	async function refreshServerState() {
		if (!server) {
			return;
		}

		try {
			const [health, channelResponse, serverInfo] = await Promise.all([
				getHealth(server.baseUrl),
				getChannels(server.baseUrl),
				getServerInfo(server.baseUrl)
			]);

			backendStatus = health.status;
			channels = channelResponse.channels;
			adminPublicKeys = serverInfo.adminPublicKeys ?? [];

			server = {
				...server,
				name: serverInfo.name,
				serverFingerprint: serverInfo.serverFingerprint,
				livekitUrl: serverInfo.livekitUrl,
				channels,
				lastConnectedAt: new Date().toISOString()
			};
			upsertServer(server);

			if (currentView !== 'admin' && !currentChannelID && channels.length > 0) {
				await goto(`/server/${server.id}?view=channel&channel=${encodeURIComponent(channels[0].id)}`, {
					replaceState: true,
					noScroll: true
				});
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function openAdmin() {
		if (!server) {
			return;
		}
		await goto(`/server/${server.id}?view=admin`, { noScroll: true });
	}

	async function openChannel(channelID: string) {
		if (!server) {
			return;
		}
		await goto(`/server/${server.id}?view=channel&channel=${encodeURIComponent(channelID)}`, {
			noScroll: true
		});
	}

	async function refreshInvites() {
		if (!server || !identity || !isAdmin) {
			return;
		}

		loadingInvites = true;
		invitesError = '';
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminListInvitesSignature({
				adminPublicKey: identity.publicKey,
				issuedAt,
				adminPrivateKeyBase64: identity.privateKey
			});

			const response = await listInvitesByClient(
				{
					adminPublicKey: identity.publicKey,
					issuedAt,
					signature
				},
				server.baseUrl
			);
			invites = response.invites;
		} catch (e) {
			invitesError = e instanceof Error ? e.message : 'Failed to load invites';
		} finally {
			invitesLoaded = true;
			loadingInvites = false;
		}
	}

	async function handleCreateInvite() {
		if (!server || !identity) {
			createInviteError = 'Identity is not available';
			return;
		}
		if (!targetClientPublicKey.trim()) {
			createInviteError = 'Target client public key is required';
			return;
		}

		creatingInvite = true;
		createInviteError = '';
		createdInviteLink = '';
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminInviteSignature({
				adminPublicKey: identity.publicKey,
				clientPublicKey: targetClientPublicKey.trim(),
				issuedAt,
				adminPrivateKeyBase64: identity.privateKey
			});

			const result = await createInviteByClient(
				{
					adminPublicKey: identity.publicKey,
					clientPublicKey: targetClientPublicKey.trim(),
					label: targetClientLabel.trim(),
					issuedAt,
					signature
				},
				server.baseUrl
			);
			createdInviteLink = result.inviteLink;
			targetClientPublicKey = '';
			targetClientLabel = '';
			await refreshInvites();
		} catch (e) {
			createInviteError = e instanceof Error ? e.message : 'Failed to create invite';
		} finally {
			creatingInvite = false;
		}
	}

	$: if (currentView === 'admin' && isAdmin && !loading && !invitesLoaded && !loadingInvites) {
		void refreshInvites();
	}

	$: if (initialized) {
		const serverID = $page.params.id;
		if (serverID && serverID !== activeServerID) {
			void initializeServer(serverID);
		}
	}
</script>

{#if server}
	<div class="server-layout">
		<aside class="channel-sidebar">
			<header class="server-header">
				<h1>{server.name}</h1>
				<p>{server.serverFingerprint}</p>
			</header>

			<section class="sidebar-group">
				<h2>Channels</h2>
				{#if channels.length === 0}
					<p class="muted">No channels available</p>
				{:else}
					{#each channels as channel}
						<button
							class="nav-item"
							class:active={currentView !== 'admin' && selectedChannel?.id === channel.id}
							on:click={() => openChannel(channel.id)}
						>
							<span class="icon">{channel.type === 'voice' ? '🔊' : '#'}</span>
							<span>{channel.name}</span>
						</button>
					{/each}
				{/if}
			</section>

			<section class="sidebar-group">
				<h2>Pages</h2>
				<button class="nav-item" class:active={currentView === 'admin'} on:click={openAdmin}>
					<span class="icon">🛠</span>
					<span>Admin</span>
				</button>
			</section>
		</aside>

		<section class="server-content">
			{#if loading}
				<p>Loading server data...</p>
			{:else if error}
				<p class="error">{error}</p>
			{:else if currentView === 'admin'}
				<h2>Admin</h2>
				{#if !isAdmin}
					<p class="error">You are not a server administrator.</p>
				{:else}
					<section class="card">
						<h3>Create Invite</h3>
						<label for="target-key">Client public key</label>
						<textarea
							id="target-key"
							bind:value={targetClientPublicKey}
							rows="4"
							placeholder="Base64 Ed25519 public key"
						></textarea>
						<label for="target-label">Label (optional)</label>
						<input id="target-label" bind:value={targetClientLabel} placeholder="Optional label" />
						<div class="actions-row">
							<button on:click={handleCreateInvite} disabled={creatingInvite || !targetClientPublicKey.trim()}>
								{creatingInvite ? 'Creating...' : 'Create Invite'}
							</button>
							<button class="ghost" on:click={refreshInvites} disabled={loadingInvites}>Refresh</button>
						</div>
						{#if createInviteError}
							<p class="error">{createInviteError}</p>
						{/if}
						{#if createdInviteLink}
							<p>Invite link:</p>
							<code class="block">{createdInviteLink}</code>
						{/if}
					</section>

					<section class="card">
						<h3>Invites</h3>
						{#if loadingInvites}
							<p>Loading invites...</p>
						{:else if invitesError}
							<p class="error">{invitesError}</p>
						{:else if invites.length === 0}
							<p>No invites yet.</p>
						{:else}
							<table>
								<thead>
									<tr>
										<th>Status</th>
										<th>Invite ID</th>
										<th>Label</th>
										<th>Client Key</th>
										<th>Created</th>
									</tr>
								</thead>
								<tbody>
									{#each invites as invite}
										<tr>
											<td>
												<span class:used={invite.status === 'used'}>{invite.status}</span>
											</td>
											<td><code>{invite.inviteId}</code></td>
											<td>{invite.label || '-'}</td>
											<td><code>{invite.allowedClientPublicKey}</code></td>
											<td>{invite.createdAt}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</section>
				{/if}
			{:else}
				<h2>{selectedChannel ? selectedChannel.name : 'Channel'}</h2>
				<p class="muted">
					Type: <strong>{selectedChannel?.type ?? 'unknown'}</strong>
				</p>
				<section class="card">
					<p>Backend status: <strong class={backendStatus === 'ok' ? 'ok' : 'fail'}>{backendStatus}</strong></p>
					<p>Server fingerprint: <code>{server.serverFingerprint}</code></p>
					<p>LiveKit URL: <code>{server.livekitUrl}</code></p>
					<p>Channel content is not implemented yet.</p>
				</section>
			{/if}
		</section>
	</div>
{/if}

<style>
	.server-layout {
		display: grid;
		grid-template-columns: 280px minmax(0, 1fr);
		gap: 0;
		background: #111724;
		border: 1px solid #273149;
		border-radius: 12px;
		min-height: calc(100vh - 120px);
		overflow: hidden;
	}

	.channel-sidebar {
		background: #1b2233;
		border-right: 1px solid #273149;
		padding: 12px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.server-header h1 {
		margin: 0;
		font-size: 18px;
	}

	.server-header p {
		margin: 4px 0 0;
		color: #9fb1cf;
	}

	.sidebar-group h2 {
		margin: 0 0 8px;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #8ea5c9;
	}

	.nav-item {
		width: 100%;
		text-align: left;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: transparent;
		color: #d6deee;
		cursor: pointer;
		margin-bottom: 4px;
	}

	.nav-item:hover {
		background: #25304a;
	}

	.nav-item.active {
		background: #2f63ff;
		border-color: #83a3ff;
		color: #ffffff;
	}

	.icon {
		width: 20px;
		text-align: center;
	}

	.server-content {
		padding: 16px;
		overflow: auto;
	}

	.card {
		margin-top: 12px;
		padding: 14px;
		border-radius: 10px;
		border: 1px solid #2f3c58;
		background: #151c2b;
	}

	.actions-row {
		display: flex;
		gap: 8px;
		margin-top: 8px;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #2f63ff;
		color: white;
		cursor: pointer;
	}

	button.ghost {
		background: #25304a;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	textarea,
	input {
		width: 100%;
		box-sizing: border-box;
		padding: 8px;
		margin-top: 6px;
		margin-bottom: 8px;
		background: #0f1521;
		border: 1px solid #2f3c58;
		border-radius: 8px;
		color: #e7eefc;
		font-family: inherit;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	th,
	td {
		padding: 8px;
		border-bottom: 1px solid #2f3c58;
		text-align: left;
		vertical-align: top;
	}

	th {
		color: #9fb1cf;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	code.block {
		display: block;
		padding: 8px;
		border-radius: 8px;
		background: #0e1420;
		word-break: break-all;
	}

	.ok {
		color: #7ef2ab;
	}

	.fail,
	.error {
		color: #ff7d7d;
	}

	.muted {
		color: #9fb1cf;
	}

	.used {
		color: #c3d2ee;
		opacity: 0.8;
	}

	@media (max-width: 980px) {
		.server-layout {
			grid-template-columns: 1fr;
			min-height: auto;
		}

		.channel-sidebar {
			border-right: 0;
			border-bottom: 1px solid #273149;
		}
	}
</style>
