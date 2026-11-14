<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import { live } from '#lib/live.svelte.ts';
	import { deleteWorkout, getWorkout, saveWorkoutAsRoutine, updateWorkout } from '#lib/repo.ts';
	import type { Workout } from '#lib/db.ts';

	const id = $derived(page.params.id ?? '');
	const workout = live(
		() => id,
		(id) => getWorkout(id),
		undefined as Workout | undefined
	);

	async function saveAsRoutine() {
		const routineId = await saveWorkoutAsRoutine(id);
		await goto(`/routines/${routineId}`);
	}

	async function remove() {
		if (!confirm('Delete this workout?')) return;
		await deleteWorkout(id);
		await goto('/');
	}
</script>

{#if workout.current}
	{@const w = workout.current}
	<div class="card stack">
		<input
			class="title"
			aria-label="Workout name"
			value={w.name}
			onchange={(e) => {
				const v = e.currentTarget.value.trim();
				if (v) updateWorkout(id, { name: v });
			}}
		/>
		<input
			type="date"
			aria-label="Workout date"
			value={w.date}
			onchange={(e) => {
				if (e.currentTarget.value) updateWorkout(id, { date: e.currentTarget.value });
			}}
		/>
	</div>

	{#key id}
		<ExerciseEditor kind="workout" parentId={id} />
	{/key}

	<div class="row actions">
		<button onclick={saveAsRoutine}>Save as routine</button>
		<span class="spacer"></span>
		<button class="danger" onclick={remove}>Delete workout</button>
	</div>
{:else if workout.loaded}
	<p class="muted">Workout not found. <a href="/">Back to history</a></p>
{/if}

<style>
	.title {
		font-size: 1.3rem;
		font-weight: 700;
	}

	.actions {
		margin-top: 1rem;
	}
</style>
