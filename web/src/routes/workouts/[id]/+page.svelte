<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import TopBar from '#lib/TopBar.svelte';
	import { elapsed } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { deleteWorkout, getWorkout, saveWorkoutAsRoutine, updateWorkout } from '#lib/repo.ts';
	import type { Workout } from '#lib/db.ts';

	const id = $derived(page.params.id ?? '');
	const workout = live(
		() => id,
		(id) => getWorkout(id),
		undefined as Workout | undefined
	);

	// Ticks once a second while a running workout with a known start is open.
	let now = $state(Date.now());
	$effect(() => {
		const w = workout.current;
		if (!w || w.finished_at != null || !w.started_at) return;
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
		title={running && w.started_at ? elapsed(now - w.started_at) : ''}
		titleLabel={running && w.started_at ? 'Elapsed time' : undefined}
	>
		{#snippet action()}
			{#if running}
				<button class="primary finish" onclick={() => goto(`/workouts/${id}/finish`)}>Finish</button>
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
		<!-- A running workout is always today; the date is only editable once it is in history. -->
		{#if !running}
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
	</div>

	{#key id}
		<ExerciseEditor kind="workout" parentId={id} />
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

	.finish {
		min-height: 32px;
		padding: 0 0.95rem;
		border-radius: 999px;
		font-size: 0.95rem;
	}
</style>
