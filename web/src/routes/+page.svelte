<script lang="ts">
	import type { Workout } from '#lib/db.ts';
	import { goto } from '$app/navigation';
	import { plural } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import RoutineIcon from '#lib/RoutineIcon.svelte';
	import { splitLabel } from '#lib/splits.ts';
	import {
		createRoutine,
		createWorkout,
		listRoutines,
		listRunningWorkouts,
		startWorkoutFromRoutine,
		type RoutineSummary
	} from '#lib/repo.ts';

	const routines = live(listRoutines, [] as RoutineSummary[]);
	const running = live(listRunningWorkouts, [] as Workout[]);

	const todayLabel = new Date().toLocaleDateString(undefined, {
		weekday: 'long',
		day: 'numeric',
		month: 'short'
	});

	/** Stable hue per routine name, so each routine keeps its colour. */
	function hue(name: string): number {
		let h = 0;
		for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) % 360;
		return h;
	}

	/** True while a button here is creating something and navigating to it. */
	let busy = $state(false);

	// A running workout can only be finished or discarded, so opening the app goes straight back to it.
	// Skipped while starting one here, which navigates by itself.
	$effect(() => {
		const w = running.current[0];
		if (w && !busy) goto(`/workouts/${w.id}`, { replaceState: true });
	});

	/** Create something and open it, ignoring repeat taps. Failures reach the error toast. */
	async function open(create: () => Promise<string>, path: string) {
		if (busy) return;
		busy = true;
		try {
			const id = await create();
			await goto(`${path}/${id}`);
		} finally {
			busy = false;
		}
	}

	const startEmpty = () => open(() => createWorkout(), '/workouts');
	const newRoutine = () => open(() => createRoutine(), '/routines');
	const start = (id: string) => open(() => startWorkoutFromRoutine(id), '/workouts');
</script>

<p class="eyebrow">{todayLabel}</p>
<h1 class="large-title">Workout</h1>

<div class="section">
	<h2>Routines</h2>
	{#if routines.current.length > 0}<span class="count">{routines.current.length}</span>{/if}
	<span class="spacer"></span>
	<button class="new" aria-label="New routine" onclick={newRoutine}>
		<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v12M6 12h12" /></svg>
		New
	</button>
</div>

{#if routines.loaded && routines.current.length === 0}
	<button class="empty" onclick={newRoutine}>
		<strong>No routines yet</strong>
		<span>Create one here, or save a finished workout as a routine.</span>
	</button>
{:else}
	<ul class="routines">
		{#each routines.current as r (r.id)}
			<li style="--h: {hue(r.name)}">
				<a href="/routines/{r.id}">
					<span class="avatar" aria-hidden="true"><RoutineIcon split={r.split} /></span>
					<span class="info">
						<span class="name">{r.name}</span>
						<span class="meta">
							{#if splitLabel(r.split, r.split_name)}{splitLabel(r.split, r.split_name)} ·
							{/if}
							{#if r.exercise_count > 0}
								<b>{r.exercise_count}</b>
								{plural(r.exercise_count, 'exercise')}
							{:else}
								No exercises yet
							{/if}
						</span>
					</span>
				</a>
				<button class="edit" aria-label="Edit {r.name}" onclick={() => goto(`/routines/${r.id}`)}>
					<svg viewBox="0 0 24 24" aria-hidden="true"
						><path d="M4 20h4L19 9a2.83 2.83 0 0 0-4-4L4 16v4z" /><path d="M13.5 6.5l4 4" /></svg
					>
				</button>
				<button class="play" aria-label="Start {r.name}" disabled={busy} onclick={() => start(r.id)}>
					<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5.5v13l10.5-6.5z" /></svg>
				</button>
			</li>
		{/each}
	</ul>
{/if}

<button class="empty-workout" disabled={busy} onclick={startEmpty}>
	<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v12M6 12h12" /></svg>
	Start empty workout
</button>

<style>
	svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.eyebrow {
		margin: 0.25rem 0 0;
		font-size: 0.78rem;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
	}

	.section {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		margin: 0.25rem 0 0.6rem;
	}

	.section h2 {
		font-size: 1.05rem;
		font-weight: 700;
	}

	.count {
		min-width: 20px;
		padding: 0 6px;
		border-radius: 999px;
		background: var(--surface);
		color: var(--muted);
		font-size: 0.75rem;
		font-weight: 600;
		line-height: 20px;
		text-align: center;
	}

	.new {
		display: flex;
		align-items: center;
		gap: 0.2rem;
		min-height: 34px;
		padding: 0 0.8rem 0 0.6rem;
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		font-size: 0.85rem;
		font-weight: 600;
	}

	.new svg {
		width: 18px;
		height: 18px;
	}

	.empty {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		width: 100%;
		padding: 1.1rem;
		border: 1.5px dashed var(--border);
		border-radius: 16px;
		background: transparent;
		text-align: left;
		font-size: 0.9rem;
	}

	.empty span {
		color: var(--muted);
	}

	.routines {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.routines li {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.7rem 0.7rem 0.7rem 0.8rem;
		border-radius: 16px;
		background: var(--surface);
	}

	.routines a {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		color: inherit;
	}

	.avatar {
		flex-shrink: 0;
		display: grid;
		place-items: center;
		width: 46px;
		height: 46px;
		border-radius: 13px;
		background: hsl(var(--h) 70% 55% / 0.18);
		color: hsl(var(--h) 75% 60%);
		padding: 5px;
	}

	.info {
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	.name {
		font-size: 0.98rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		font-size: 0.8rem;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta b {
		color: var(--text);
		font-weight: 600;
	}

	.edit,
	.play {
		flex-shrink: 0;
		display: grid;
		place-items: center;
		width: 44px;
		height: 44px;
		min-height: 0;
		padding: 0;
		border-radius: 50%;
		background: var(--accent);
		color: var(--accent-text);
		transition: transform 0.12s;
	}

	.edit:active,
	.play:active {
		transform: scale(0.92);
	}

	/* Secondary to Start: tinted, not filled. */
	.edit {
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
	}

	.edit svg {
		width: 20px;
		height: 20px;
		stroke-width: 2;
	}

	.play svg {
		width: 20px;
		height: 20px;
		fill: currentColor;
		stroke-width: 1.5;
		margin-left: 2px;
	}

	/* Full-width, so it reads as a real action rather than a footnote. */
	.empty-workout {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		width: 100%;
		margin: 1rem 0 0;
		min-height: 52px;
		padding: 0 1rem;
		border: 1.5px dashed var(--border);
		border-radius: 16px;
		background: transparent;
		color: var(--text);
		font-size: 1rem;
		font-weight: 600;
	}

	.empty-workout svg {
		width: 18px;
		height: 18px;
		color: var(--accent);
	}
</style>
