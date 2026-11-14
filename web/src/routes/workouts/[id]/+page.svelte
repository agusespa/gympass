<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import TopBar from '#lib/TopBar.svelte';
	import { live } from '#lib/live.svelte.ts';
	import {
		deleteWorkout,
		finishWorkout,
		getWorkout,
		resumeWorkout,
		saveWorkoutAsRoutine,
		updateWorkout
	} from '#lib/repo.ts';
	import { showToast } from '#lib/toast.svelte.ts';
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

	/** No confirmation: finishing is one tap, and the toast offers an undo instead. */
	async function finish() {
		const finished = id;
		await finishWorkout(finished);
		await goto('/profile');
		showToast({
			message: 'Workout finished',
			action: {
				label: 'Undo',
				run: async () => {
					await resumeWorkout(finished);
					await goto(`/workouts/${finished}`);
				}
			}
		});
	}

	/** Delete; a discarded workout that was still running goes back to the start screen. */
	async function remove(to: string) {
		await deleteWorkout(id);
		await goto(to);
	}
</script>

{#if workout.current}
	{@const w = workout.current}
	{@const running = w.finished_at == null}
	<TopBar title={running ? 'Log workout' : 'Workout'} fallback={running ? '/' : '/profile'}>
		{#snippet action()}
			{#if running}
				<button class="primary finish" onclick={finish}>Finish</button>
			{/if}
		{/snippet}
	</TopBar>
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
		{#if running}
			<ConfirmButton label="Discard workout" confirmLabel="Confirm discard" onconfirm={() => remove('/')} />
		{:else}
			<ConfirmButton label="Delete workout" onconfirm={() => remove('/profile')} />
		{/if}
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

	.finish {
		min-height: 32px;
		padding: 0 0.95rem;
		border-radius: 999px;
		font-size: 0.95rem;
	}
</style>
