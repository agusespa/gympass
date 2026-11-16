<script lang="ts">
	import { goto } from '$app/navigation';
	import type { Snippet } from 'svelte';

	/**
	 * Sticky screen header with a back button; `fallback` is used when there is no history to go back to.
	 * `action` renders on the right (e.g. a Finish button). `back={false}` hides the back button.
	 * `progress` (0 to 1) draws a line that far along the bar's bottom edge.
	 */
	let {
		title = '',
		fallback,
		titleLabel,
		back: showBack = true,
		action,
		progress
	}: {
		title?: string;
		/** Accessible name for the title, e.g. "Elapsed time" for a running workout's clock. */
		titleLabel?: string;
		fallback: string;
		back?: boolean;
		action?: Snippet;
		progress?: number;
	} = $props();

	function back() {
		if (history.length > 1) history.back();
		else goto(fallback);
	}
</script>

<div class="topbar">
	{#if showBack}
		<button class="back" aria-label="Back" onclick={back}>
			<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
		</button>
	{/if}
	<!-- No whitespace around the text, so the timer reads exactly "0:42". -->
	<span class="title" role={titleLabel ? 'timer' : undefined} aria-label={titleLabel}
		>{#if titleLabel && title}<svg class="clock" viewBox="0 0 24 24" aria-hidden="true"
				><circle cx="12" cy="13.5" r="7.5" /><path d="M12 13.5V10M10 2.5h4M12 2.5V6M18.5 6.5l1.5-1.5" /></svg
			>{/if}{title}</span
	>
	{#if action}
		<span class="action">{@render action()}</span>
	{/if}
	{#if progress !== undefined}
		<!-- Decorative: whoever passes it also says the same in words. -->
		<span class="progress" data-testid="progress-line" style:width="{progress * 100}%" aria-hidden="true"
		></span>
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
		/* Extra room at the top keeps the back button and title below iOS's own status bar blur. */
		padding: calc(env(safe-area-inset-top) + 1.8rem) 1rem 0.35rem;
		background: var(--bar);
		-webkit-backdrop-filter: var(--bar-filter);
		backdrop-filter: var(--bar-filter);
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

	/* Keeps the bar the same height when there is no back button. */
	.title {
		flex: 1;
		display: flex;
		align-items: center;
		min-height: 44px;
		font-size: 1.15rem;
		font-weight: 500;
		/* Digits keep their width so a ticking clock doesn't jitter. */
		font-variant-numeric: tabular-nums;
	}

	.clock {
		width: 20px;
		height: 20px;
		margin-right: 0.4rem;
		fill: none;
		stroke: var(--accent);
		stroke-width: 2.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	/* Over the bar's bottom border. */
	.progress {
		position: absolute;
		left: 0;
		bottom: -1px;
		height: 2px;
		background: var(--accent);
		transition: width 0.3s ease;
	}

	.action {
		display: flex;
		margin-right: -0.25rem;
	}
</style>
