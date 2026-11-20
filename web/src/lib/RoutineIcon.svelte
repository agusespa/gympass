<script lang="ts">
	import { HIGHLIGHTS, type BodyPart, type RoutineIcon } from './routine-icons.ts';

	/** A routine's icon: a figure with the trained parts lit, or a dumbbell when none is chosen. */
	let { icon }: { icon: RoutineIcon | '' } = $props();

	const lit = $derived(new Set<BodyPart>(icon ? HIGHLIGHTS[icon] : []));
	const fromBehind = $derived(lit.has('back'));
	const op = (part: BodyPart) => (lit.has(part) ? 1 : 0.25);
</script>

{#if icon}
	<svg class="figure" viewBox="0 0 24 24" aria-hidden="true">
		<circle cx="12" cy="3.6" r="2.1" opacity={icon === 'full' ? 1 : 0.25} />
		<g opacity={op('shoulders')}>
			<ellipse cx="7.4" cy="7.9" rx="1.7" ry="1.5" />
			<ellipse cx="16.6" cy="7.9" rx="1.7" ry="1.5" />
		</g>
		{#if fromBehind}
			<!-- Lats and upper back as one V. -->
			<path
				opacity={op('back')}
				d="M9 6.6h6c.5 0 .8.4.6.9l-1.8 6.8c-.1.4-.4.6-.8.6h-2c-.4 0-.7-.2-.8-.6L8.4 7.5c-.2-.5.1-.9.6-.9z"
			/>
		{:else}
			<g opacity={op('chest')}>
				<rect x="9.3" y="6.6" width="2.5" height="3.6" rx="1" />
				<rect x="12.2" y="6.6" width="2.5" height="3.6" rx="1" />
			</g>
			<rect opacity={op('core')} x="9.6" y="10.8" width="4.8" height="4" rx="1.1" />
		{/if}
		<g opacity={op('arms')}>
			<rect x="5.4" y="9.7" width="2" height="6.2" rx="1" />
			<rect x="16.6" y="9.7" width="2" height="6.2" rx="1" />
		</g>
		<g opacity={op('legs')}>
			<rect x="9.4" y="15.6" width="2.3" height="7" rx="1.1" />
			<rect x="12.3" y="15.6" width="2.3" height="7" rx="1.1" />
		</g>
	</svg>
{:else}
	<svg class="dumbbell" viewBox="0 0 24 24" aria-hidden="true">
		<path d="M6.5 7v10M17.5 7v10M3.5 9.5v5M20.5 9.5v5M6.5 12h11" />
	</svg>
{/if}

<style>
	svg {
		width: 100%;
		height: 100%;
	}

	.figure {
		fill: currentColor;
	}

	.dumbbell {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.4;
		stroke-linecap: round;
	}
</style>
