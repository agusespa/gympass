<script lang="ts">
	import { goto } from '$app/navigation';

	/** Sticky screen header with a back button; `fallback` is used when there is no history to go back to. */
	let { title, fallback }: { title: string; fallback: string } = $props();

	function back() {
		if (history.length > 1) history.back();
		else goto(fallback);
	}
</script>

<div class="topbar">
	<button class="back" aria-label="Back" onclick={back}>
		<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
	</button>
	<span class="title">{title}</span>
</div>

<style>
	.topbar {
		position: sticky;
		top: 0;
		z-index: 5;
		display: flex;
		align-items: center;
		gap: 0.9rem;
		margin: calc(-1 * (env(safe-area-inset-top) + 1rem)) -1rem 0;
		padding: calc(env(safe-area-inset-top) + 0.6rem) 1rem 0.6rem;
		background: var(--bar);
		-webkit-backdrop-filter: saturate(180%) blur(20px);
		backdrop-filter: saturate(180%) blur(20px);
		border-bottom: 1px solid var(--border);
	}

	.back {
		display: grid;
		place-items: center;
		width: 44px;
		height: 44px;
		padding: 0;
		border-radius: 50%;
		background: var(--surface);
		box-shadow: 0 0 0 1px var(--border);
	}

	.back svg {
		width: 22px;
		height: 22px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.title {
		font-size: 1.15rem;
		font-weight: 500;
	}
</style>
