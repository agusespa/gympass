<script lang="ts">
	import { shortDate } from './format';
	import type { Week } from './stats';

	/**
	 * Workouts per week as bars, oldest first. Tapping or hovering a bar shows that week.
	 * With a goal (workouts per week), finished weeks that fell short are red.
	 */
	let { weeks, goal = 0 }: { weeks: Week[]; goal?: number } = $props();

	let picked = $state<number | null>(null);
	const index = $derived(picked ?? weeks.length - 1);
	const shown = $derived(weeks[index]);
	// Scaled so a week that met the goal reaches at least the goal's height.
	const max = $derived(Math.max(goal, ...weeks.map((w) => w.workouts)));
	const missed = (i: number) => goal > 0 && i < weeks.length - 1 && weeks[i].workouts < goal;
	const count = (w: Week) =>
		goal > 0
			? `${w.workouts} / ${goal} workouts`
			: `${w.workouts} ${w.workouts === 1 ? 'workout' : 'workouts'}`;
	const label = (w: Week, i: number) =>
		`Week of ${shortDate(w.start)}: ${count(w)}${missed(i) ? ', goal missed' : ''}`;
</script>

<section class="card chart" aria-label="Weekly workouts">
	<div class="head">
		<span class="muted">{index === weeks.length - 1 ? 'This week' : `Week of ${shortDate(shown.start)}`}</span
		>
		<strong class:missed={missed(index)} data-testid="week-workouts">{count(shown)}</strong>
	</div>
	<div class="plot" role="group" aria-label="Workouts per week, last {weeks.length} weeks">
		{#each weeks as w, i (w.start)}
			<button
				class="col"
				class:selected={i === index}
				class:missed={missed(i)}
				aria-label={label(w, i)}
				aria-pressed={i === index}
				onclick={() => (picked = i)}
				onpointerenter={(e) => e.pointerType === 'mouse' && (picked = i)}
			>
				<span class="bar" class:empty={w.workouts === 0} style:height="{max ? (w.workouts / max) * 100 : 0}%"
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
		font-size: 1.1rem;
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

	.col.missed .bar {
		background: color-mix(in srgb, var(--danger) 45%, transparent);
	}

	.col.missed .bar.empty {
		background: color-mix(in srgb, var(--danger) 60%, transparent);
	}

	.col.missed.selected .bar {
		background: var(--danger);
	}

	.head strong.missed {
		color: var(--danger);
	}

	.axis {
		display: flex;
		justify-content: space-between;
		padding-top: 0.35rem;
		font-size: 0.72rem;
	}
</style>
