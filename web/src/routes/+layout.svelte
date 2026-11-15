<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { installErrorToasts } from '#lib/errors.ts';
	import PullToRefresh from '#lib/PullToRefresh.svelte';
	import { startSync, sync } from '#lib/sync.svelte.ts';
	import { runToastAction, toast } from '#lib/toast.svelte.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	onMount(() => {
		installErrorToasts();
		startSync();
	});

	/**
	 * Offline support and record ids need a secure context, so plain http:// (e.g. a LAN IP)
	 * is not supported; say so instead of failing on the first tap.
	 */
	const insecure = typeof window !== 'undefined' && !window.isSecureContext;

	const onProfile = $derived(page.url.pathname.startsWith('/profile'));
	/** Tabs show only on the two tab screens; workout and routine screens use their back button. */
	const showTabs = $derived(page.url.pathname === '/' || page.url.pathname === '/profile');

	const TABS = ['/', '/profile'];

	// Switching tabs slides the page toward the tab that was tapped (Safari 18+, Chrome);
	// browsers without view transitions just switch.
	onNavigate((nav) => {
		const from = TABS.indexOf(nav.from?.url.pathname ?? '');
		const to = TABS.indexOf(nav.to?.url.pathname ?? '');
		if (from < 0 || to < 0 || from === to || !document.startViewTransition) return;
		if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		document.documentElement.dataset.tabDir = to > from ? 'forward' : 'back';
		return new Promise((resolve) => {
			const t = document.startViewTransition(async () => {
				resolve();
				await nav.complete;
			});
			t.finished.finally(() => delete document.documentElement.dataset.tabDir);
		});
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Gym</title>
</svelte:head>

<main class:with-tabs={showTabs}>
	{#if insecure}
		<div class="banner" role="alert">
			<span>This app needs a secure connection. Open it through its https:// address.</span>
		</div>
	{/if}
	{#if sync.status === 'auth-expired'}
		<div class="banner" role="alert">
			<span>Your login has expired. Your data is saved on this phone.</span>
			<button class="primary" onclick={() => location.reload()}>Reload</button>
		</div>
	{/if}
	{#if sync.rejected > 0}
		<div class="banner" role="alert">
			<span>
				{sync.rejected === 1 ? '1 change' : `${sync.rejected} changes`} couldn't be saved to the server. They're
				kept on this phone.
			</span>
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
	<!-- Only on the tab screens: a pull on a workout screen would get in the way of editing sets. -->
	<PullToRefresh />
	<nav class="tabbar" style="--tab: {onProfile ? 1 : 0}">
		<span class="indicator" aria-hidden="true"></span>
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
		-webkit-backdrop-filter: var(--bar-filter);
		backdrop-filter: var(--bar-filter);
		box-shadow: var(--bar-edge), var(--bar-shadow);
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
		width: min(calc(100% - 48px), 300px);
		/* Gap between the bar's edge and the active tab's pill. */
		--inset: 3px;
		padding: var(--inset);
		/* Kept out of the page slide; only its live state is shown so the pill can glide. */
		view-transition-name: tabbar;
		border-radius: 999px;
		background: var(--bar);
		-webkit-backdrop-filter: var(--bar-filter);
		backdrop-filter: var(--bar-filter);
		box-shadow: var(--bar-edge), var(--bar-shadow);
	}

	.tabbar a {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		position: relative;
		z-index: 1;
		gap: 4px;
		padding: 7px 0 8px;
		border-radius: 999px;
		color: var(--text);
		font-size: 13px;
		font-weight: 500;
		line-height: 1;
		transition:
			color 0.3s,
			transform 0.15s;
		-webkit-user-select: none;
		user-select: none;
	}

	/* viewBox is cropped to the drawing so padding alone sets the spacing. */
	.tabbar a:active {
		transform: scale(0.94);
	}

	.tabbar svg {
		width: 29px;
		height: 24px;
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
	}

	/* The active tab's background, sliding under whichever tab is selected. */
	.indicator {
		position: absolute;
		top: var(--inset);
		bottom: var(--inset);
		left: var(--inset);
		width: calc(50% - var(--inset));
		border-radius: 999px;
		background: var(--bar-active);
		transform: translateX(calc(var(--tab) * 100%));
		transition: transform 0.38s cubic-bezier(0.3, 1.3, 0.5, 1);
	}

	@media (prefers-reduced-motion: reduce) {
		.indicator {
			transition: none;
		}
	}
</style>
