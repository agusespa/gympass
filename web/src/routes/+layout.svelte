<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { startSync, sync } from '#lib/sync.svelte.ts';
	import { runToastAction, toast } from '#lib/toast.svelte.ts';
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

{#if toast.current}
	{@const t = toast.current}
	<div class="toast" class:above-tabs={showTabs} role="status">
		<span>{t.message}</span>
		{#if t.action}
			<button onclick={runToastAction}>{t.action.label}</button>
		{/if}
	</div>
{/if}

{#if showTabs}
	<nav class="tabbar">
		<a href="/" class:active={!onProfile} aria-current={!onProfile ? 'page' : undefined}>
			<!-- The app icon's dumbbell (scripts/icons.mjs) scaled down, filled like the icon. -->
			<svg class="solid" viewBox="1 3.5 26 21.5" aria-hidden="true">
				<rect x="7.94" y="12.86" width="12.12" height="2.78" rx="0.63" />
				<rect x="1.5" y="10.72" width="2.4" height="7.07" rx="1.01" />
				<rect x="4.28" y="6.93" width="4.04" height="14.64" rx="1.39" />
				<rect x="19.68" y="6.93" width="4.04" height="14.64" rx="1.39" />
				<rect x="24.1" y="10.72" width="2.4" height="7.07" rx="1.01" />
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

	.toast {
		position: fixed;
		left: 50%;
		bottom: calc(env(safe-area-inset-bottom) + 16px);
		transform: translateX(-50%);
		z-index: 20;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: min(calc(100% - 32px), 400px);
		padding: 0.4rem 0.4rem 0.4rem 1rem;
		border-radius: 14px;
		background: var(--bar);
		-webkit-backdrop-filter: saturate(180%) blur(20px);
		backdrop-filter: saturate(180%) blur(20px);
		box-shadow: var(--bar-shadow);
		font-size: 0.95rem;
	}

	.toast.above-tabs {
		bottom: calc(env(safe-area-inset-bottom) + 84px);
	}

	.toast span {
		flex: 1;
	}

	.toast button {
		background: transparent;
		color: var(--accent);
		font-weight: 600;
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

	.tabbar svg.solid {
		fill: currentColor;
		stroke: none;
	}

	.tabbar a.active {
		color: var(--accent);
		background: var(--bar-active);
	}
</style>
