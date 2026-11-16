<script lang="ts">
	import type { Equipment } from './catalog';

	let { equipment }: { equipment: Equipment } = $props();
</script>

<svg viewBox="0 0 24 24" aria-hidden="true">
	{#if equipment === 'barbell'}
		<path d="M1.5 12h21M6 6.5v11M9 8.5v7M18 6.5v11M15 8.5v7" />
	{:else if equipment === 'dumbbell'}
		<rect x="3" y="7.5" width="4.5" height="9" rx="1.5" />
		<rect x="16.5" y="7.5" width="4.5" height="9" rx="1.5" />
		<path d="M7.5 12h9" />
	{:else if equipment === 'machine'}
		<rect x="6" y="3" width="12" height="18" rx="1.5" />
		<path d="M6 9.5h12M6 13.5h12M6 17.5h12M12 3v6.5" />
	{:else if equipment === 'cable'}
		<circle cx="7.5" cy="6.5" r="3.5" />
		<path d="M11 6.5h5v8M12.5 19.5l3.5-5 3.5 5z" />
	{:else if equipment === 'kettlebell'}
		<circle cx="12" cy="15" r="6" />
		<path d="M9 9.8V7a3 3 0 0 1 6 0v2.8" />
	{:else}
		<circle cx="12" cy="4.5" r="2" />
		<path d="M5 9l7 1.5L19 9M12 10.5V15l-3 6.5M12 15l3 6.5" />
	{/if}
</svg>

<style>
	svg {
		width: 22px;
		height: 22px;
		flex: none;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.75;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
