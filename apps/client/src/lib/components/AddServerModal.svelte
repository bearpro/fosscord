<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { connectBegin, connectFinish } from '$lib/api';
	import { createHandshakeSignature } from '$lib/crypto';
	import { parseInviteLink } from '$lib/invite';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	export let open = false;
	export let identity: IdentityRecord;
	export let allowedBaseURL: string | null = null;

	const dispatch = createEventDispatcher<{
		close: void;
		connected: SavedServer;
	}>();

	let inviteLink = '';
	let loading = false;
	let error = '';

	async function handleConnect() {
		error = '';
		loading = true;

		try {
			const parsedInvite = parseInviteLink(inviteLink);
			const targetBaseURL = parsedInvite.baseUrl.replace(/\/$/, '');
			const normalizedAllowedBaseURL = allowedBaseURL?.replace(/\/$/, '') ?? null;
			if (normalizedAllowedBaseURL && targetBaseURL !== normalizedAllowedBaseURL) {
				throw new Error(
					`invite base URL mismatch: expected ${normalizedAllowedBaseURL}, got ${targetBaseURL}`
				);
			}

			const connectBaseURL = normalizedAllowedBaseURL ?? targetBaseURL;
			const begin = await connectBegin(parsedInvite.inviteID, connectBaseURL);

			if (begin.serverFingerprint !== parsedInvite.serverFingerprint) {
				throw new Error(
					`server fingerprint mismatch: expected ${parsedInvite.serverFingerprint}, got ${begin.serverFingerprint}`
				);
			}

			const signature = await createHandshakeSignature({
				challengeBase64: begin.challenge,
				inviteID: parsedInvite.inviteID,
				serverFingerprint: begin.serverFingerprint,
				clientPrivateKeyBase64: identity.privateKey
			});

			const finish = await connectFinish(
				{
					inviteId: parsedInvite.inviteID,
					clientPublicKey: identity.publicKey,
					challenge: begin.challenge,
					signature,
					clientInfo: {
						displayName: 'Desktop Client'
					}
				},
				connectBaseURL
			);

			dispatch('connected', {
				id: finish.serverId,
				name: finish.serverName,
				baseUrl: connectBaseURL,
				serverFingerprint: finish.serverFingerprint,
				livekitUrl: finish.livekitUrl,
				sessionToken: finish.sessionToken,
				channels: finish.channels,
				lastConnectedAt: new Date().toISOString()
			});

			inviteLink = '';
			dispatch('close');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}
</script>

{#if open}
	<div class="overlay" role="presentation">
		<div class="modal">
			<h2>Add server</h2>
			<p>Paste invite link (`fw://connect?...` or URL with query params).</p>
			<textarea
				bind:value={inviteLink}
				rows="5"
				placeholder="fw://connect?baseUrl=http%3A%2F%2Flocalhost%3A8080&inviteId=...&serverFp=..."
			></textarea>

			{#if error}
				<p class="error">{error}</p>
			{/if}

			<div class="actions">
				<button type="button" on:click={() => dispatch('close')} disabled={loading}>Cancel</button>
				<button type="button" on:click={handleConnect} disabled={loading || !inviteLink.trim()}>
					{loading ? 'Connecting...' : 'Connect'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(4, 8, 14, 0.72);
		display: grid;
		place-items: center;
		padding: 16px;
		z-index: 80;
	}

	.modal {
		width: min(680px, 100%);
		background: #151c2b;
		border: 1px solid #2f3c58;
		border-radius: 12px;
		padding: 16px;
		box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
		color: #e7eefc;
	}

	h2 {
		margin: 0 0 8px;
	}

	p {
		margin: 0 0 10px;
		color: #9fb1cf;
	}

	textarea {
		width: 100%;
		box-sizing: border-box;
		margin-top: 8px;
		padding: 10px;
		font-family: inherit;
		background: #0f1521;
		border: 1px solid #2f3c58;
		border-radius: 8px;
		color: #e7eefc;
		resize: vertical;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 12px;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #2f63ff;
		color: white;
		cursor: pointer;
	}

	button:first-child {
		background: #25304a;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: #ff7d7d;
		margin-top: 10px;
	}
</style>
