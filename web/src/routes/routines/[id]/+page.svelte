<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import { live } from '#lib/live.svelte.ts';
	import { deleteRoutine, getRoutine, startWorkoutFromRoutine, updateRoutine } from '#lib/repo.ts';
	import type { Routine } from '#lib/db.ts';

	const id = $derived(page.params.id ?? '');
	const routine = live(
		() => id,
		(id) => getRoutine(id),
		undefined as Routine | undefined
	);

	async function start() {
		const workoutId = await startWorkoutFromRoutine(id);
		await goto(`/workouts/${workoutId}`);
	}

	async function remove() {
		if (!confirm('Delete this routine?')) return;
		await deleteRoutine(id);
		await goto('/routines');
	}
</script>

{#if routine.current}
	<div class="card row">
		<input
			class="title"
			aria-label="Routine name"
			value={routine.current.name}
			onchange={(e) => {
				const v = e.currentTarget.value.trim();
				if (v) updateRoutine(id, { name: v });
			}}
		/>
		<button class="primary" onclick={start}>Start</button>
	</div>

	{#key id}
		<ExerciseEditor kind="routine" parentId={id} />
	{/key}

	<div class="row actions">
		<span class="spacer"></span>
		<button class="danger" onclick={remove}>Delete routine</button>
	</div>
{:else if routine.loaded}
	<p class="muted">Routine not found. <a href="/routines">Back to routines</a></p>
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
