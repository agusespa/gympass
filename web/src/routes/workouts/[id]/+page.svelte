<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import NameInput from '#lib/NameInput.svelte';
	import TopBar from '#lib/TopBar.svelte';
	import { elapsed } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import {
		deleteWorkout,
		getWorkout,
		loadExercises,
		saveWorkoutAsRoutine,
		updateWorkout,
		type ExerciseView
	} from '#lib/repo.ts';
	import type { Workout } from '#lib/db.ts';

	const id = $derived(page.params.id ?? '');
	const workout = live(
		() => id,
		(id) => getWorkout(id),
		undefined as Workout | undefined
	);

	const exercises = live(
		() => id,
		(id) => loadExercises('workout', id),
		[] as ExerciseView[]
	);
	/** Exercises with every set checked off: the ones the editor folds away. */
	const finished = $derived(
		exercises.current.filter((e) => e.sets.length > 0 && e.sets.every((s) => s.done)).length
	);

	/** Share of all sets, warm-ups included, that are checked off. */
	const setsDone = $derived.by(() => {
		const sets = exercises.current.flatMap((e) => e.sets);
		return sets.length > 0 ? sets.filter((s) => s.done).length / sets.length : 0;
	});

	// Ticks once a second while a running workout is open.
	let now = $state(Date.now());
	$effect(() => {
		const w = workout.current;
		if (!w || w.finished_at != null) return;
		now = Date.now();
		const timer = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(timer);
	});

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
	{@const running = w.finished_at == null}
	<!-- A running workout is left by finishing or discarding it, not by going back. -->
	<TopBar
		fallback="/profile"
		back={!running}
		title={running ? elapsed(now - w.started_at) : ''}
		titleLabel={running ? 'Elapsed time' : undefined}
		progress={running ? setsDone : undefined}
	>
		{#snippet action()}
			{#if running && exercises.current.length > 0}
				<span class="progress" aria-label="{finished} of {exercises.current.length} exercises done">
					{finished}/{exercises.current.length}
				</span>
			{/if}
			{#if running}
				<button class="primary finish" onclick={() => goto(`/workouts/${id}/finish`)}>Finish</button>
			{/if}
		{/snippet}
		<!-- Running: the name shares the clock's row so the bar stays one line; a long name gets "…". -->
		{#snippet center()}
			{#if running}
				<NameInput
					compact
					label="Workout name"
					value={w.name}
					onsave={(name) => updateWorkout(id, { name })}
				/>
			{/if}
		{/snippet}
		<!-- Finished: name and date under the back button. A running workout is always today. -->
		{#if !running}
			<NameInput compact label="Workout name" value={w.name} onsave={(name) => updateWorkout(id, { name })} />
			<input
				class="date"
				type="date"
				aria-label="Workout date"
				value={w.date}
				onchange={(e) => {
					if (e.currentTarget.value) updateWorkout(id, { date: e.currentTarget.value });
				}}
			/>
		{/if}
	</TopBar>

	{#key id}
		<ExerciseEditor kind="workout" parentId={id} collapseDone={running} />
	{/key}

	<!-- A running workout is saved or discarded from the Finish screen. -->
	{#if !running}
		<div class="row page-actions">
			<button onclick={saveAsRoutine}>Save as routine</button>
			<span class="spacer"></span>
			<ConfirmButton label="Delete workout" onconfirm={remove} />
		</div>
	{/if}
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

	.progress {
		align-self: center;
		margin-right: 0.75rem;
		color: var(--muted);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}

	.finish {
		min-height: 32px;
		padding: 0 0.95rem;
		border-radius: 999px;
		font-size: 0.95rem;
	}
</style>
