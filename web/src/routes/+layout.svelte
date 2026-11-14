<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const onRoutines = $derived(page.url.pathname.startsWith('/routines'));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Gym</title>
</svelte:head>

<main>
	{@render children()}
</main>

<nav>
	<a href="/" class:active={!onRoutines}>History</a>
	<a href="/routines" class:active={onRoutines}>Routines</a>
</nav>

<style>
	main {
		max-width: 640px;
		margin: 0 auto;
		padding: calc(env(safe-area-inset-top) + 1rem) 1rem
			calc(env(safe-area-inset-bottom) + 5rem);
	}

	nav {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		display: flex;
		background: var(--surface);
		border-top: 1px solid var(--border);
		padding-bottom: env(safe-area-inset-bottom);
	}

	nav a {
		flex: 1;
		text-align: center;
		padding: 0.9rem 0;
		color: var(--muted);
		font-weight: 600;
	}

	nav a.active {
		color: var(--accent);
	}
</style>
