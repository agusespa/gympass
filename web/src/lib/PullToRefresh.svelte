<script lang="ts">
	import { onMount } from 'svelte';

	/**
	 * Pull down from the top of the page to reload, like a browser tab. Only in the home-screen
	 * app: Safari and Chrome tabs have their own, and two would reload twice. A reload picks up a
	 * new app version, renews an expired login and syncs on start.
	 */

	/** How far (px) the indicator has to come down before letting go reloads. */
	const THRESHOLD = 72;
	const MAX = 110;
	/** Finger travel (px) ignored before a pull starts, so taps and small jitters don't count. */
	const SLOP = 6;

	let pull = $state(0);
	let dragging = $state(false);
	let refreshing = $state(false);
	const ready = $derived(pull >= THRESHOLD);

	onMount(() => {
		const standalone =
			(navigator as Navigator & { standalone?: boolean }).standalone === true ||
			matchMedia('(display-mode: standalone)').matches;
		if (!standalone) return;

		let startY: number | null = null;

		const reset = () => {
			startY = null;
			dragging = false;
			pull = 0;
		};

		const start = (e: TouchEvent) => {
			const atTop = scrollY <= 0;
			const busy = refreshing || e.touches.length !== 1 || document.querySelector('dialog[open]');
			startY = atTop && !busy ? e.touches[0].clientY : null;
		};

		const move = (e: TouchEvent) => {
			if (startY === null) return;
			const dy = e.touches[0].clientY - startY;
			if (!dragging) {
				// Moving up, or the page already scrolled: this is a normal scroll.
				if (dy < 0 || scrollY > 0) return void (startY = null);
				if (dy < SLOP) return;
				dragging = true;
			}
			// Stops the iOS rubber band so the page stays put while the indicator comes down.
			e.preventDefault();
			pull = Math.min(MAX, Math.max(0, dy - SLOP) * 0.5);
		};

		const end = () => {
			if (!dragging) return void (startY = null);
			if (pull < THRESHOLD) return reset();
			startY = null;
			dragging = false;
			pull = THRESHOLD;
			refreshing = true;
			location.reload();
		};

		addEventListener('touchstart', start, { passive: true });
		addEventListener('touchmove', move, { passive: false });
		addEventListener('touchend', end);
		addEventListener('touchcancel', reset);
		return () => {
			removeEventListener('touchstart', start);
			removeEventListener('touchmove', move);
			removeEventListener('touchend', end);
			removeEventListener('touchcancel', reset);
		};
	});
</script>

<div
	class="ptr"
	class:dragging
	class:ready
	class:refreshing
	style:--pull="{pull}px"
	style:--progress={Math.min(pull / THRESHOLD, 1)}
	data-testid="pull-to-refresh"
	aria-hidden="true"
>
	<svg viewBox="0 0 24 24">
		<path d="M20 12a8 8 0 1 1-2.34-5.66" />
		<path d="M20 4.5v4h-4" />
	</svg>
</div>

<style>
	.ptr {
		position: fixed;
		top: env(safe-area-inset-top);
		left: 50%;
		z-index: 30;
		display: grid;
		place-items: center;
		width: 40px;
		height: 40px;
		border-radius: 999px;
		background: var(--bar);
		-webkit-backdrop-filter: var(--bar-filter);
		backdrop-filter: var(--bar-filter);
		box-shadow: var(--bar-edge), var(--bar-shadow);
		color: var(--muted);
		opacity: var(--progress);
		transform: translate(-50%, calc(var(--pull) - 44px));
		transition:
			transform 0.3s cubic-bezier(0.3, 1.3, 0.5, 1),
			opacity 0.2s;
		pointer-events: none;
	}

	/* Follow the finger exactly while pulling; ease back only on release. */
	.ptr.dragging {
		transition: none;
	}

	.ptr.ready,
	.ptr.refreshing {
		color: var(--accent);
	}

	svg {
		width: 22px;
		height: 22px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2.2;
		stroke-linecap: round;
		stroke-linejoin: round;
		transform: rotate(calc(var(--progress) * 270deg));
	}

	.refreshing svg {
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(1turn);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.ptr {
			transition: none;
		}

		.refreshing svg {
			animation: none;
		}
	}
</style>
