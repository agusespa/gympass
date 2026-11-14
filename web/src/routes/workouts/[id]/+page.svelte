<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import TopBar from '#lib/TopBar.svelte';
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
		await deleteWorkout(id);
		await goto('/profile');
	}
</script>

{#if workout.current}
	{@const w = workout.current}
	<TopBar title="Log workout" fallback="/profile" />
	<div class="page-details">
		<input
			class="page-name"
			aria-label="Workout name"
			value={w.name}
			onchange={(e) => {
				const v = e.currentTarget.value.trim();
				if (v) updateWorkout(id, { name: v });
			}}
		/>
		<input
			class="date"
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

	<div class="row page-actions">
		<button onclick={saveAsRoutine}>Save as routine</button>
		<span class="spacer"></span>
		<ConfirmButton label="Delete workout" onconfirm={remove} />
	</div>
{:else if workout.loaded}
	<p class="muted">Workout not found. <a href="/profile">Back to history</a></p>
{/if}

<style>
	.date {
		width: auto;
		background: var(--surface);
		border-color: transparent;
		margin-top: 0.4rem;
	}
</style>
