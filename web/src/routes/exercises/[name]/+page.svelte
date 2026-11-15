<script lang="ts">
	import { page } from '$app/state';
	import ProgressChart from '#lib/ProgressChart.svelte';
	import TopBar from '#lib/TopBar.svelte';
	import { durationLabel, kg, longDate, setLabel } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { setBodyweightPct } from '#lib/repo.ts';
	import {
		exerciseSessions,
		exerciseStats,
		loadBodyContext,
		loadHistory,
		type BodyContext,
		type History
	} from '#lib/stats.ts';

	const key = $derived(page.params.name ?? '');
	const history = live(
		() => null,
		() => loadHistory(),
		{ workoutDates: [], performances: [] } as History
	);

	const stats = $derived(exerciseStats(history.current).find((x) => x.key === key));
	const sessions = $derived(exerciseSessions(history.current, key));
	const timed = $derived(stats?.timed ?? false);
	// Bodyweight exercises have no weight to estimate a max from: chart reps (or time) instead.
	const weighted = $derived(sessions.some((s) => s.bestE1rm > 0));
	const points = $derived(
		[...sessions]
			.reverse()
			.map((s) => ({
				date: s.date,
				value: weighted
					? s.bestE1rm
					: Math.max(0, ...s.sets.map((x) => (timed ? x.duration_seconds : x.reps)))
			}))
			.filter((p) => p.value > 0)
	);
	const chartLabel = $derived(weighted ? 'Est. 1RM' : timed ? 'Longest' : 'Most reps');
	const chartFormat = $derived(
		weighted ? (v: number) => `${oneDecimal(v)} kg` : timed ? durationLabel : (v: number) => `${v} reps`
	);

	const body = live(
		() => null,
		() => loadBodyContext(),
		{ weights: [], shares: new Map() } as BodyContext
	);
	const pct = $derived(body.current.shares.get(key) ?? 0);
	const SHARES = [
		{ pct: 0, label: 'Not counted' },
		{ pct: 65, label: '65% · push-ups' },
		{ pct: 100, label: '100% · pull-ups, dips' }
	];
	const shareOptions = $derived(
		SHARES.some((o) => o.pct === pct)
			? SHARES
			: [...SHARES, { pct, label: `${pct}%` }].sort((a, b) => a.pct - b.pct)
	);

	const oneDecimal = (v: number) => kg(Math.round(v * 10) / 10);
</script>

<TopBar fallback="/profile" title={stats?.name ?? ''} />

{#if stats}
	<dl class="stats" aria-label="Bests">
		{#if weighted}
			<div>
				<dt>Heaviest</dt>
				<dd data-testid="best-weight">
					{kg(stats.heaviest?.set.weight_kg ?? 0)} kg<small
						>× {timed
							? durationLabel(stats.heaviest?.set.duration_seconds ?? 0)
							: stats.heaviest?.set.reps}</small
					>
				</dd>
			</div>
			<div>
				<dt>Est. 1RM</dt>
				<dd data-testid="best-e1rm">{oneDecimal(stats.bestE1rm?.value ?? 0)} kg</dd>
			</div>
		{/if}
		{#if timed}
			<div>
				<dt>Longest</dt>
				<dd data-testid="best-time">{durationLabel(stats.longest?.set.duration_seconds ?? 0)}</dd>
			</div>
		{:else}
			<div>
				<dt>Most reps</dt>
				<dd>{stats.mostReps?.set.reps ?? 0}</dd>
			</div>
		{/if}
		<div>
			<dt>Sessions</dt>
			<dd>{stats.sessions}</dd>
		</div>
	</dl>

	{#if points.length > 1}
		<ProgressChart {points} label={chartLabel} format={chartFormat} />
		{#if weighted}
			<p class="muted note">Estimated from your best set each session (weight × reps, up to 12 reps).</p>
		{/if}
	{/if}

	<div class="card setting">
		<label for="bw-share">Body weight</label>
		<select
			id="bw-share"
			value={pct}
			onchange={(e) => stats && setBodyweightPct(stats.name, Number(e.currentTarget.value))}
		>
			{#each shareOptions as o (o.pct)}
				<option value={o.pct}>{o.label}</option>
			{/each}
		</select>
	</div>
	{#if pct > 0 && body.loaded && body.current.weights.length === 0}
		<p class="muted note">Log your body weight in <a href="/profile/settings">Settings</a> to count it.</p>
	{/if}

	<h2 class="section-title">History</h2>
	{#each sessions as s (s.workoutId + s.date)}
		<a class="card session" href="/workouts/{s.workoutId}">
			<span class="when">
				<strong>{longDate(s.date)}</strong>
				<span class="muted">{s.workoutName}</span>
			</span>
			<span class="sets">
				{#each s.sets as set, i (i)}
					<span>{setLabel(set, timed)}</span>
				{/each}
			</span>
		</a>
	{/each}
{:else if history.loaded}
	<p class="muted empty">No finished sets of this exercise yet. <a href="/profile">Back to profile</a></p>
{/if}

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
		grid-auto-flow: column;
		gap: 0.5rem;
		margin: 1rem 0 0.75rem;
	}

	.stats div {
		background: var(--surface);
		border-radius: var(--radius);
		padding: 0.7rem 0.4rem;
		text-align: center;
		min-width: 0;
	}

	.stats dt {
		font-size: 0.75rem;
		color: var(--muted);
	}

	.stats dd {
		margin: 0.2rem 0 0;
		font-size: 1.1rem;
		font-weight: 600;
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}

	.stats small {
		display: block;
		font-size: 0.8rem;
		font-weight: 500;
		color: var(--muted);
	}

	.note {
		font-size: 0.8rem;
		margin: -0.25rem 0.25rem 0;
	}

	.setting {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		margin-top: 0.75rem;
	}

	.setting select {
		min-width: 0;
		max-width: 60%;
		background: none;
		border: none;
		color: var(--accent);
		text-align: right;
	}

	.section-title {
		font-size: 1.3rem;
		font-weight: 600;
		margin: 1.25rem 0 0.75rem;
	}

	.session {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		color: inherit;
	}

	.session:active {
		opacity: 0.7;
	}

	.when {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		min-width: 0;
	}

	.when .muted {
		font-size: 0.88rem;
	}

	.sets {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.1rem;
		font-size: 0.92rem;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.empty {
		text-align: center;
		margin-top: 2rem;
	}
</style>
