<script lang="ts">
	import { goto } from '$app/navigation';
	import { live } from '#lib/live.svelte.ts';
	import { createRoutine, listRoutines, startWorkoutFromRoutine } from '#lib/repo.ts';
	import type { Routine } from '#lib/db.ts';

	const routines = live(
		() => null,
		() => listRoutines(),
		[] as Routine[]
	);

	async function create() {
		const id = await createRoutine();
		await goto(`/routines/${id}`);
	}

	async function start(id: string) {
		const workoutId = await startWorkoutFromRoutine(id);
		await goto(`/workouts/${workoutId}`);
	}
</script>

<div class="row">
	<h1>Routines</h1>
	<span class="spacer"></span>
	<button class="primary" onclick={create}>New routine</button>
</div>

{#if routines.loaded && routines.current.length === 0}
	<p class="muted">No routines yet. Create one, or save a workout as a routine.</p>
{/if}

<ul>
	{#each routines.current as r (r.id)}
		<li class="card row">
			<a href="/routines/{r.id}"><strong>{r.name}</strong></a>
			<span class="spacer"></span>
			<button class="primary" aria-label="Start {r.name}" onclick={() => start(r.id)}>Start</button>
		</li>
	{/each}
</ul>

<style>
	ul {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	li a {
		color: inherit;
		flex: 1;
		padding: 0.5rem 0;
	}
</style>
