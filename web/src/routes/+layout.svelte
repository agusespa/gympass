<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { startSync, sync } from '#lib/sync.svelte.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	onMount(startSync);

	const onProfile = $derived(page.url.pathname.startsWith('/profile'));
	/** Tabs show only on the two tab screens; workout and routine screens use their back button. */
	const showTabs = $derived(page.url.pathname === '/' || onProfile);
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Gym</title>
</svelte:head>

<main class:with-tabs={showTabs}>
	{#if sync.status === 'auth-expired'}
		<div class="banner" role="alert">
			<span>Your login has expired. Your data is saved on this phone.</span>
			<button class="primary" onclick={() => location.reload()}>Reload</button>
		</div>
	{/if}
	{@render children()}
</main>

{#if showTabs}
	<nav class="tabbar">
		<a href="/" class:active={!onProfile} aria-current={!onProfile ? 'page' : undefined}>
			<svg viewBox="1 3.5 26 21.5" aria-hidden="true">
				<rect x="5.25" y="8.25" width="2.5" height="11.5" rx="1.25" />
				<rect x="20.25" y="8.25" width="2.5" height="11.5" rx="1.25" />
				<rect x="9.25" y="5.75" width="2.5" height="16.5" rx="1.25" />
				<rect x="16.25" y="5.75" width="2.5" height="16.5" rx="1.25" />
				<path d="M11.75 14h4.5M2.5 14h2.75M22.75 14h2.75" />
			</svg>
			<span>Workout</span>
		</a>
		<a href="/profile" class:active={onProfile} aria-current={onProfile ? 'page' : undefined}>
			<svg viewBox="1 3.5 26 21.5" aria-hidden="true">
				<circle cx="14" cy="9.5" r="4.75" />
				<path d="M5 24c.8-4.6 4.5-7.5 9-7.5s8.2 2.9 9 7.5" />
			</svg>
			<span>Profile</span>
		</a>
	</nav>
{/if}

<style>
	main {
		max-width: 640px;
		margin: 0 auto;
		padding: calc(env(safe-area-inset-top) + 1rem) 1rem calc(env(safe-area-inset-bottom) + 2rem);
	}

	main.with-tabs {
		padding-bottom: calc(env(safe-area-inset-bottom) + 6rem);
	}

	.banner {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		background: var(--surface);
		border: 1px solid var(--danger);
		border-radius: var(--radius);
		padding: 0.75rem;
		margin-bottom: 0.75rem;
	}

	/* Floating pill tab bar, centered above the home indicator. */
	.tabbar {
		position: fixed;
		left: 50%;
		bottom: calc(env(safe-area-inset-bottom) + 8px);
		transform: translateX(-50%);
		z-index: 10;
		display: flex;
		width: min(calc(100% - 48px), 280px);
		padding: 5px;
		border-radius: 999px;
		background: var(--bar);
		-webkit-backdrop-filter: saturate(180%) blur(20px);
		backdrop-filter: saturate(180%) blur(20px);
		box-shadow: var(--bar-shadow);
	}

	.tabbar a {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		padding: 5px 0 6px;
		border-radius: 999px;
		color: var(--text);
		font-size: 12px;
		font-weight: 500;
		line-height: 1;
		transition:
			background-color 0.2s,
			color 0.2s;
		-webkit-user-select: none;
		user-select: none;
	}

	/* viewBox is cropped to the drawing so padding alone sets the spacing. */
	.tabbar svg {
		width: 26px;
		height: 21.5px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.7;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.tabbar a.active {
		color: var(--accent);
		background: var(--bar-active);
	}
</style>
