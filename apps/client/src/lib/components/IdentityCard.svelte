<script lang="ts">
	export let title = 'Identity';
	export let fingerprint = '';
	export let publicKey = '';
</script>

<section class="identity-card">
	<h2>{title}</h2>
	<p class="fingerprint">{fingerprint || 'n/a'}</p>
	{#if publicKey}
		<p class="meta">Public key: {publicKey}</p>
	{/if}
</section>

<style>
	.identity-card {
		padding: 16px;
		border-radius: 12px;
		background: #ffffff;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.08);
	}

	.fingerprint {
		font-size: 24px;
		margin: 8px 0;
	}

	.meta {
		margin: 0;
		font-size: 12px;
		word-break: break-all;
		color: #334155;
	}
</style>
