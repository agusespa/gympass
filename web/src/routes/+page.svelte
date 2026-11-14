<script lang="ts">
	import { goto } from '$app/navigation';
	import { live } from '#lib/live.svelte.ts';
	import {
		createRoutine,
		createWorkout,
		listRoutines,
		startWorkoutFromRoutine,
		type RoutineSummary
	} from '#lib/repo.ts';

	const routines = live(
		() => null,
		() => listRoutines(),
		[] as RoutineSummary[]
	);

	async function startEmpty() {
		const id = await createWorkout();
		await goto(`/workouts/${id}`);
	}

	async function newRoutine() {
		const id = await createRoutine();
		await goto(`/routines/${id}`);
	}

	async function start(id: string) {
		const workoutId = await startWorkoutFromRoutine(id);
		await goto(`/workouts/${workoutId}`);
	}
</script>

<h1 class="large-title">Workout</h1>

<button class="tile" onclick={startEmpty}>
	<svg viewBox="0 0 24 24" aria-hidden="true">
		<path d="M12 4v16M4 12h16" />
	</svg>
	Start empty workout
</button>

<h2 class="section-title">Routines</h2>

<button class="tile" onclick={newRoutine}>
	<svg viewBox="0 0 24 24" aria-hidden="true">
		<rect x="5" y="4.5" width="14" height="16.5" rx="2.5" />
		<rect x="9" y="2.75" width="6" height="3.5" rx="1" />
		<path d="M9 11h6M9 15h6" />
	</svg>
	New routine
</button>

{#if routines.loaded && routines.current.length === 0}
	<p class="muted">No routines yet. Create one, or save a workout as a routine.</p>
{:else if routines.current.length > 0}
	<p class="group-label">My routines ({routines.current.length})</p>
	{#each routines.current as r (r.id)}
		<div class="card routine">
			<a href="/routines/{r.id}">
				<strong>{r.name}</strong>
				{#if r.exercises.length > 0}
					<span class="muted summary">{r.exercises.join(', ')}</span>
				{/if}
			</a>
			<button class="primary start" aria-label="Start {r.name}" onclick={() => start(r.id)}
				>Start routine</button
			>
		</div>
	{/each}
{/if}

<style>
	.tile {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		min-height: 56px;
		padding: 0 1.1rem;
		margin-bottom: 0.75rem;
		background: var(--surface);
		border-radius: var(--radius);
		font-size: 1.05rem;
		font-weight: 500;
		text-align: left;
	}

	.tile svg {
		width: 24px;
		height: 24px;
		flex-shrink: 0;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.6;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.section-title {
		font-size: 1.3rem;
		font-weight: 600;
		margin: 1.5rem 0 0.75rem;
	}

	.group-label {
		color: var(--muted);
		margin: 1.25rem 0 0.75rem;
	}

	.routine {
		padding: 1.1rem;
	}

	.routine a {
		display: block;
		color: inherit;
	}

	.routine strong {
		display: block;
		font-size: 1.15rem;
		font-weight: 600;
	}

	.summary {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		margin-top: 0.3rem;
	}

	.start {
		width: 100%;
		min-height: 46px;
		margin-top: 0.9rem;
		font-size: 1.05rem;
		font-weight: 500;
	}
</style>
