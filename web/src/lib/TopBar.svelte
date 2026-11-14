<script lang="ts">
	import { goto } from '$app/navigation';
	import type { Snippet } from 'svelte';

	/**
	 * Sticky screen header with a back button; `fallback` is used when there is no history to go back to.
	 * `action` renders on the right (e.g. a Finish button).
	 */
	let { title, fallback, action }: { title: string; fallback: string; action?: Snippet } = $props();

	function back() {
		if (history.length > 1) history.back();
		else goto(fallback);
	}
</script>

<div class="topbar">
	<button class="back" aria-label="Back" onclick={back}>
		<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
	</button>
	<span class="title">{title}</span>
	{#if action}
		<span class="action">{@render action()}</span>
	{/if}
</div>

<style>
	.topbar {
		position: sticky;
		top: 0;
		z-index: 5;
		display: flex;
		align-items: center;
		gap: 0.25rem;
		margin: calc(-1 * (env(safe-area-inset-top) + 1rem)) -1rem 0;
		padding: calc(env(safe-area-inset-top) + 0.6rem) 1rem 0.6rem;
		background: var(--bar);
		-webkit-backdrop-filter: saturate(180%) blur(20px);
		backdrop-filter: saturate(180%) blur(20px);
		border-bottom: 1px solid var(--border);
	}

	/* iOS-style back chevron: no background, tinted like a link. */
	.back {
		display: grid;
		place-items: center;
		width: 36px;
		height: 44px;
		min-height: 0;
		margin-left: -0.5rem;
		padding: 0;
		background: transparent;
		color: var(--accent);
	}

	.back svg {
		width: 26px;
		height: 26px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2.4;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.title {
		flex: 1;
		font-size: 1.15rem;
		font-weight: 500;
	}

	.action {
		display: flex;
		margin-right: -0.25rem;
	}
</style>
