<script lang="ts">
	import { shortDate, tonnage } from './format';
	import type { Week } from './stats';

	/** Weekly volume bars, oldest first. Tapping or hovering a bar shows that week. */
	let { weeks }: { weeks: Week[] } = $props();

	let picked = $state<number | null>(null);
	const index = $derived(picked ?? weeks.length - 1);
	const shown = $derived(weeks[index]);
	const max = $derived(Math.max(0, ...weeks.map((w) => w.volume)));
	const label = (w: Week) =>
		`Week of ${shortDate(w.start)}: ${tonnage(w.volume)}, ${w.workouts} ${w.workouts === 1 ? 'workout' : 'workouts'}`;
</script>

<section class="card chart" aria-label="Weekly volume">
	<div class="head">
		<span class="muted">{index === weeks.length - 1 ? 'This week' : `Week of ${shortDate(shown.start)}`}</span
		>
		<strong data-testid="week-volume">{tonnage(shown.volume)}</strong>
		<span class="muted">{shown.workouts} {shown.workouts === 1 ? 'workout' : 'workouts'}</span>
	</div>
	<div class="plot" role="group" aria-label="Volume per week, last {weeks.length} weeks">
		{#each weeks as w, i (w.start)}
			<button
				class="col"
				class:selected={i === index}
				aria-label={label(w)}
				aria-pressed={i === index}
				onclick={() => (picked = i)}
				onpointerenter={(e) => e.pointerType === 'mouse' && (picked = i)}
			>
				<span class="bar" class:empty={w.volume === 0} style:height="{max ? (w.volume / max) * 100 : 0}%"
				></span>
			</button>
		{/each}
	</div>
	<div class="axis muted" aria-hidden="true">
		<span>{shortDate(weeks[0].start)}</span>
		<span>{shortDate(weeks[Math.floor(weeks.length / 2)].start)}</span>
		<span>This week</span>
	</div>
</section>

<style>
	.chart {
		padding: 0.9rem 0.9rem 0.6rem;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
		font-variant-numeric: tabular-nums;
	}

	.head strong {
		font-size: 1.25rem;
		font-weight: 700;
	}

	.head .muted {
		font-size: 0.85rem;
	}

	.head .muted:first-child {
		margin-right: auto;
	}

	.plot {
		display: flex;
		align-items: stretch;
		gap: 2px;
		height: 110px;
		border-bottom: 1px solid var(--separator);
	}

	/* The whole column is the tap target, bigger than the bar. */
	.col {
		flex: 1;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		min-height: 0;
		padding: 0 2px;
		background: none;
		border-radius: 0;
	}

	.bar {
		display: block;
		width: 100%;
		max-width: 22px;
		border-radius: 4px 4px 0 0;
		background: color-mix(in srgb, var(--accent) 40%, transparent);
		transition: background-color 0.15s;
	}

	/* A sliver marks weeks with nothing logged, so the gap reads as zero rather than missing. */
	.bar.empty {
		height: 2px !important;
		background: var(--border);
	}

	.col.selected .bar:not(.empty) {
		background: var(--accent);
	}

	.col.selected .bar.empty {
		background: var(--muted);
	}

	.axis {
		display: flex;
		justify-content: space-between;
		padding-top: 0.35rem;
		font-size: 0.72rem;
	}
</style>
