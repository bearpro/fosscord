<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { loadIdentity } from '$lib/storage';

	onMount(() => {
		if (loadIdentity()) {
			void goto('/servers');
			return;
		}
		void goto('/setup');
	});
</script>

<p>Redirecting...</p>
