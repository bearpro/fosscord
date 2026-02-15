<script lang="ts">
	export let title = 'Identity';
	export let fingerprint = '';
	export let publicKey = '';
</script>

<section class="identity-card">
	<h2>{title}</h2>
	<p class="fingerprint">{fingerprint || 'n/a'}</p>
	{#if publicKey}
		<p class="label">Public Key</p>
		<code class="key">{publicKey}</code>
	{/if}
</section>

<style>
	.identity-card {
		padding: 16px;
		border-radius: 12px;
		background: #151c2b;
		border: 1px solid #2f3c58;
	}

	h2 {
		margin: 0;
		color: #e7eefc;
		font-size: 18px;
	}

	.fingerprint {
		font-size: 24px;
		margin: 10px 0;
	}

	.label {
		margin: 0 0 6px;
		color: #9fb1cf;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.key {
		display: block;
		padding: 10px;
		border-radius: 8px;
		background: #0f1521;
		border: 1px solid #2f3c58;
		margin: 0;
		font-size: 12px;
		word-break: break-all;
		color: #d3dff6;
	}
</style>
