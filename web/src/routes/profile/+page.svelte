<script lang="ts">
	import type { Workout } from '#lib/db.ts';
	import { duration, durationLabel, kg, longDate, setLabel, shortDate } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { getWeeklyGoal, listWorkouts, today } from '#lib/repo.ts';
	import {
		behindGoal,
		exerciseStats,
		loadHistory,
		recentRecords,
		summary,
		weekly,
		type ExerciseStats,
		type History,
		type RecentRecord
	} from '#lib/stats.ts';
	import { needsSync } from '#lib/sync.svelte.ts';
	import WeeklyChart from '#lib/WeeklyChart.svelte';

	const workouts = live(
		() => null,
		() => listWorkouts(),
		[] as Workout[]
	);
	const history = live(
		() => null,
		() => loadHistory(),
		{ workoutDates: [], performances: [] } as History
	);

	const goal = live(
		() => null,
		() => getWeeklyGoal(),
		0
	);

	const todayIso = today();
	const stats = $derived(summary(history.current, todayIso, goal.current));
	const goalMet = $derived(goal.current > 0 && stats.thisWeek >= goal.current);
	const behind = $derived(goal.current > 0 && behindGoal(stats.thisWeek, goal.current, todayIso));
	const weeks = $derived(weekly(history.current, todayIso));
	const exercises = $derived(exerciseStats(history.current));
	const records = $derived(recentRecords(exercises, todayIso).slice(0, 5));

	/** Records from the last two weeks get a "New" badge. */
	const newSince = (() => {
		const d = new Date();
		d.setDate(d.getDate() - 14);
		return iso(d);
	})();

	function recordText(r: RecentRecord): string {
		const s = r.record.set;
		if (r.kind === 'reps') return `${s.reps} reps`;
		if (r.kind === 'time') return durationLabel(s.duration_seconds);
		if (r.kind === 'e1rm') return `${kg(Math.round(r.record.value * 10) / 10)} kg est. 1RM`;
		return `${kg(s.weight_kg)} kg × ${s.reps}`;
	}

	function exerciseLine(x: ExerciseStats): string {
		const sessions = `${x.sessions} ${x.sessions === 1 ? 'session' : 'sessions'}`;
		const h = x.heaviest?.set;
		if (h?.weight_kg) return `${sessions} · best ${setLabel(h, x.timed)}`;
		if (x.timed)
			return x.longest ? `${sessions} · best ${durationLabel(x.longest.set.duration_seconds)}` : sessions;
		if (x.mostReps) return `${sessions} · best ${x.mostReps.set.reps} reps`;
		return sessions;
	}

	const exerciseHref = (key: string) => `/exercises/${encodeURIComponent(key)}`;
	let selected = $state(todayIso);
	// First day of the month on screen.
	let month = $state(new Date(Number(todayIso.slice(0, 4)), Number(todayIso.slice(5, 7)) - 1, 1));

	function iso(d: Date): string {
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	const byDate = $derived.by(() => {
		const map = new Map<string, Workout[]>();
		for (const w of workouts.current) {
			const list = map.get(w.date);
			if (list) list.push(w);
			else map.set(w.date, [w]);
		}
		return map;
	});

	// Weeks start on Monday; leading blanks pad the first week.
	const cells = $derived.by(() => {
		const lead = (month.getDay() + 6) % 7;
		const days = new Date(month.getFullYear(), month.getMonth() + 1, 0).getDate();
		const out: (string | null)[] = Array(lead).fill(null);
		for (let d = 1; d <= days; d++) out.push(iso(new Date(month.getFullYear(), month.getMonth(), d)));
		return out;
	});

	const monthLabel = $derived(month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }));
	const monthCount = $derived(cells.reduce((n, d) => n + (d ? (byDate.get(d)?.length ?? 0) : 0), 0));
	const weekdays = Array.from({ length: 7 }, (_, i) =>
		// 2024-01-01 was a Monday.
		new Date(2024, 0, 1 + i).toLocaleDateString(undefined, { weekday: 'narrow' })
	);
	const sessions = $derived(byDate.get(selected) ?? []);

	function shift(by: number) {
		month = new Date(month.getFullYear(), month.getMonth() + by, 1);
	}
</script>

<div class="title-row">
	<h1 class="large-title">Profile</h1>
	<a class="settings" href="/profile/settings" aria-label="Settings">
		<svg viewBox="0 0 24 24" aria-hidden="true">
			<circle cx="12" cy="12" r="3" />
			<path
				d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
			/>
		</svg>
		{#if needsSync()}
			<span class="sync-dot" data-testid="sync-badge" aria-hidden="true"></span>
		{/if}
	</a>
</div>

<dl class="stats" aria-label="Summary">
	<div>
		<dt>This week</dt>
		<dd data-testid="stat-week" class:met={goalMet} class:behind>
			{stats.thisWeek}{#if goal.current}<span class="of"> / {goal.current}</span>{/if}
		</dd>
	</div>
	<div>
		<dt>Streak</dt>
		<dd data-testid="stat-streak">{stats.streakWeeks} {stats.streakWeeks === 1 ? 'wk' : 'wks'}</dd>
	</div>
	<div>
		<dt>Avg / week</dt>
		<dd data-testid="stat-avg">
			{stats.avgPerWeek == null ? '–' : (Math.round(stats.avgPerWeek * 10) / 10).toLocaleString()}
		</dd>
	</div>
</dl>

<section class="card calendar" aria-label="Workout calendar">
	<div class="head">
		<button class="nav" aria-label="Previous month" onclick={() => shift(-1)}>‹</button>
		<div class="title">
			<strong>{monthLabel}</strong>
			<span class="muted">{monthCount} {monthCount === 1 ? 'workout' : 'workouts'}</span>
		</div>
		<button class="nav" aria-label="Next month" onclick={() => shift(1)}>›</button>
	</div>
	<div class="grid">
		{#each weekdays as wd, i (i)}
			<span class="weekday muted" aria-hidden="true">{wd}</span>
		{/each}
		{#each cells as d, i (d ?? `blank-${i}`)}
			{#if d}
				{@const count = byDate.get(d)?.length ?? 0}
				<button
					class="day"
					data-date={d}
					class:trained={count > 0}
					class:today={d === todayIso}
					class:selected={d === selected}
					aria-pressed={d === selected}
					aria-label="{longDate(d)}{count ? `, ${count} ${count === 1 ? 'workout' : 'workouts'}` : ''}"
					onclick={() => (selected = d)}
				>
					{Number(d.slice(8))}
				</button>
			{:else}
				<span></span>
			{/if}
		{/each}
	</div>
</section>

<h2 class="section-title">{selected === todayIso ? 'Today' : longDate(selected)}</h2>

{#if workouts.loaded && sessions.length === 0}
	<p class="muted">No workouts this day.</p>
{/if}

{#each sessions as w (w.id)}
	<a class="card workout" href="/workouts/{w.id}">
		<strong>
			{w.name}
			{#if w.finished_at == null}<span class="badge">In progress</span>{/if}
		</strong>
		{#if w.finished_at && duration(w.finished_at - w.started_at)}
			<span class="muted date">{duration(w.finished_at - w.started_at)}</span>
		{/if}
	</a>
{/each}

<h2 class="section-title">Weekly workouts</h2>
<WeeklyChart {weeks} goal={goal.current} />

<h2 class="section-title">Records</h2>
{#if history.loaded && records.length === 0}
	<p class="muted">Beat a previous best and it shows up here.</p>
{:else if records.length > 0}
	<ul class="card list" aria-label="Recent records">
		{#each records as r (r.key + r.kind)}
			<li>
				<a href={exerciseHref(r.key)}>
					<span class="row-main">
						<strong>{r.name}</strong>
						<span class="muted">{recordText(r)}</span>
					</span>
					<span class="row-side">
						{#if r.record.date >= newSince}<span class="badge">New</span>{/if}
						<span class="muted">{shortDate(r.record.date)}</span>
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}

<h2 class="section-title">Exercises</h2>
{#if history.loaded && exercises.length === 0}
	<p class="muted">Exercises you finish show up here.</p>
{:else if exercises.length > 0}
	<ul class="card list" aria-label="Exercises">
		{#each exercises as x (x.key)}
			<li>
				<a href={exerciseHref(x.key)}>
					<span class="row-main">
						<strong>{x.name}</strong>
						<span class="muted">{exerciseLine(x)}</span>
					</span>
					<svg class="chevron" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 5l7 7-7 7" /></svg>
				</a>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.title-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.settings {
		display: grid;
		place-items: center;
		width: 44px;
		height: 44px;
		margin: -0.5rem -0.5rem 0.5rem 0;
		color: var(--accent);
		position: relative;
	}

	.sync-dot {
		position: absolute;
		top: 8px;
		right: 7px;
		width: 9px;
		height: 9px;
		border-radius: 50%;
		background: var(--danger);
	}

	.settings svg,
	.chevron {
		width: 24px;
		height: 24px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	/* 2×2 on phones, so a big total like "12,480 kg" still fits. */
	.stats {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
		margin: 0 0 0.75rem;
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
		font-size: 1.25rem;
		font-weight: 600;
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}

	.stats dd.met {
		color: var(--accent);
	}

	.stats dd.behind {
		color: var(--danger);
	}

	.stats .of {
		margin-left: 0.15em;
		font-size: 0.9rem;
		font-weight: 500;
		opacity: 0.7;
	}

	.list {
		list-style: none;
		margin: 0 0 0.75rem;
		padding: 0 0.9rem;
	}

	.list li + li {
		border-top: 1px solid var(--border);
	}

	.list a {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.7rem 0;
		color: inherit;
	}

	.list a:active {
		opacity: 0.7;
	}

	.row-main {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
	}

	.row-main strong {
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-main .muted,
	.row-side .muted {
		font-size: 0.88rem;
	}

	.row-side {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		white-space: nowrap;
	}

	.chevron {
		width: 18px;
		height: 18px;
		color: var(--muted);
		flex: none;
	}

	.calendar {
		padding: 0.9rem 0.75rem 0.75rem;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.6rem;
	}

	.title {
		text-align: center;
	}

	.title strong {
		font-size: 1.05rem;
		text-transform: capitalize;
	}

	.title .muted {
		display: block;
		font-size: 0.8rem;
	}

	.nav {
		background: none;
		color: var(--accent);
		font-size: 1.6rem;
		line-height: 1;
		padding: 0 0.75rem;
	}

	.grid {
		display: grid;
		grid-template-columns: repeat(7, 1fr);
		gap: 0.25rem;
		justify-items: center;
	}

	.weekday {
		font-size: 0.75rem;
		font-weight: 600;
		padding-bottom: 0.2rem;
	}

	.day {
		width: 100%;
		max-width: 2.6rem;
		aspect-ratio: 1;
		min-height: 0;
		padding: 0;
		border-radius: 999px;
		background: none;
		color: var(--text);
		font-size: 0.95rem;
		font-variant-numeric: tabular-nums;
	}

	.day.trained {
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
		font-weight: 600;
	}

	.day.today {
		box-shadow: inset 0 0 0 1.5px var(--accent);
	}

	.day.selected {
		background: var(--accent);
		color: var(--accent-text);
		font-weight: 600;
	}

	.section-title {
		font-size: 1.3rem;
		font-weight: 600;
		margin: 1.25rem 0 0.75rem;
	}

	.workout {
		display: block;
		padding: 1.1rem;
		color: inherit;
	}

	.workout:active {
		opacity: 0.7;
	}

	.workout strong {
		display: block;
		font-size: 1.15rem;
		font-weight: 600;
	}

	.badge {
		margin-left: 0.35rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		font-size: 0.72rem;
		font-weight: 600;
		vertical-align: middle;
	}

	.date {
		display: block;
		font-size: 0.9rem;
		margin-top: 0.15rem;
	}
</style>
