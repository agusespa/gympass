<script lang="ts">
	import { goto } from '$app/navigation';
	import { longDate } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { createWorkout, listWorkouts, type WorkoutSummary } from '#lib/repo.ts';

	const workouts = live(
		() => null,
		() => listWorkouts(),
		[] as WorkoutSummary[]
	);

	async function start() {
		const id = await createWorkout();
		await goto(`/workouts/${id}`);
	}
</script>

<div class="row">
	<h1>History</h1>
	<span class="spacer"></span>
	<button class="primary" onclick={start}>New workout</button>
</div>

{#if workouts.loaded && workouts.current.length === 0}
	<p class="muted">No workouts yet.</p>
{/if}

<ul>
	{#each workouts.current as w (w.id)}
		<li>
			<a class="card" href="/workouts/{w.id}">
				<div class="row">
					<strong>{w.name}</strong>
					<span class="spacer"></span>
					<span class="muted">{longDate(w.date)}</span>
				</div>
				{#if w.exercises.length > 0}
					<div class="muted summary">{w.exercises.join(', ')}</div>
				{/if}
			</a>
		</li>
	{/each}
</ul>

<style>
	ul {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	a.card {
		display: block;
		color: inherit;
	}

	.summary {
		margin-top: 0.3rem;
		font-size: 0.9rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
