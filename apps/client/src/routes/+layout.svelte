<svelte:head>
	<title>Fosscord Desktop</title>
</svelte:head>

<div class="app-shell">
	<header>
		<nav>
			<a href="/">Servers</a>
			<a href="/server">Server Home</a>
		</nav>
	</header>

	<main>
		<slot />
	</main>
</div>

<style>
	:global(body) {
		margin: 0;
		font-family:
			"Iosevka",
			"JetBrains Mono",
			"Fira Code",
			monospace;
		background: linear-gradient(180deg, #f7f9fc 0%, #edf3fb 100%);
		color: #1f2937;
	}

	.app-shell {
		max-width: 960px;
		margin: 0 auto;
		padding: 24px;
	}

	header {
		margin-bottom: 24px;
	}

	nav {
		display: flex;
		gap: 16px;
	}

	a {
		color: #0f4c81;
		text-decoration: none;
		font-weight: 600;
	}

	a:hover {
		text-decoration: underline;
	}
</style>
