<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { connectBegin, connectFinish } from '$lib/api';
	import { createHandshakeSignature } from '$lib/crypto';
	import { parseInviteLink } from '$lib/invite';
	import type { IdentityRecord, SavedServer } from '$lib/types';

	export let open = false;
	export let identity: IdentityRecord;

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
			const begin = await connectBegin(parsedInvite.inviteID, parsedInvite.baseUrl);

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
				parsedInvite.baseUrl
			);

			dispatch('connected', {
				id: finish.serverId,
				name: finish.serverName,
				baseUrl: parsedInvite.baseUrl,
				serverFingerprint: finish.serverFingerprint,
				livekitUrl: finish.livekitUrl,
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
		background: rgba(15, 23, 42, 0.45);
		display: grid;
		place-items: center;
		padding: 16px;
	}

	.modal {
		width: min(680px, 100%);
		background: #ffffff;
		border-radius: 12px;
		padding: 16px;
		box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
	}

	textarea {
		width: 100%;
		box-sizing: border-box;
		margin-top: 8px;
		padding: 8px;
		font-family: inherit;
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
		background: #0f4c81;
		color: white;
		cursor: pointer;
	}

	button:first-child {
		background: #475569;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: #b91c1c;
	}
</style>
