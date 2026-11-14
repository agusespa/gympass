<script lang="ts">
	import { duration, longDate } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { listWorkouts, today, type WorkoutSummary } from '#lib/repo.ts';

	const workouts = live(
		() => null,
		() => listWorkouts(),
		[] as WorkoutSummary[]
	);

	const todayIso = today();
	let selected = $state(todayIso);
	// First day of the month on screen.
	let month = $state(new Date(Number(todayIso.slice(0, 4)), Number(todayIso.slice(5, 7)) - 1, 1));

	function iso(d: Date): string {
		return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	}

	const byDate = $derived.by(() => {
		const map = new Map<string, WorkoutSummary[]>();
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

<h1 class="large-title">Profile</h1>

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
		{#if w.started_at && w.finished_at && duration(w.finished_at - w.started_at)}
			<span class="muted date">{duration(w.finished_at - w.started_at)}</span>
		{/if}
		{#if w.exercises.length > 0}
			<span class="muted summary">{w.exercises.join(', ')}</span>
		{/if}
	</a>
{/each}

<style>
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

	.summary {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		margin-top: 0.4rem;
	}
</style>
