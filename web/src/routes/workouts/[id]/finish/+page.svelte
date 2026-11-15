<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import Modal from '#lib/Modal.svelte';
	import TopBar from '#lib/TopBar.svelte';
	import { live } from '#lib/live.svelte.ts';
	import {
		deleteWorkout,
		finishWorkout,
		getWorkout,
		loadExercises,
		resumeWorkout,
		saveWorkoutAsRoutine,
		updateWorkout,
		type ExerciseView,
		type SetView
	} from '#lib/repo.ts';
	import { bodyLoad, exerciseKey, loadBodyContext, volume, type BodyContext } from '#lib/stats.ts';
	import { showToast } from '#lib/toast.svelte.ts';
	import { durationLabel } from '#lib/format.ts';
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

	const body = live(
		() => null,
		() => loadBodyContext(),
		{ weights: [], shares: new Map() } as BodyContext
	);

	/** A set counts once it is checked off; warm-ups don't count toward the totals. */
	const logged = (s: SetView) => s.done;
	const working = (e: ExerciseView) => e.sets.filter((s) => logged(s) && !s.warmup);

	const done = $derived(exercises.current.filter((e) => e.sets.some(logged)));
	const sets = $derived(done.flatMap(working));
	const unchecked = $derived(exercises.current.reduce((n, e) => n + e.sets.filter((s) => !s.done).length, 0));
	const totals = $derived({
		exercises: done.length,
		sets: sets.length,
		reps: sets.reduce((n, s) => n + s.reps, 0),
		volume: done.reduce(
			(n, e) =>
				n + volume(working(e), bodyLoad(body.current, exerciseKey(e.name), workout.current?.date ?? '')),
			0
		)
	});

	/** Heaviest working set, ties broken by reps (or time, for timed exercises). */
	function best(e: ExerciseView): SetView | undefined {
		const effort = (s: SetView) => (e.timed ? s.duration_seconds : s.reps);
		return working(e).sort((a, b) => b.weight_kg - a.weight_kg || effort(b) - effort(a))[0];
	}

	const kg = (n: number) => `${Math.round(n * 10) / 10}`;

	/** e.g. "3 sets · best 100 kg × 5" or "3 sets · best 0:45"; bodyweight rep sets show no best. */
	function summary(e: ExerciseView): string {
		const n = working(e).length;
		const top = best(e);
		const sets = `${n} ${n === 1 ? 'set' : 'sets'}`;
		if (!top) return sets;
		if (e.timed) {
			const time = durationLabel(top.duration_seconds);
			return top.weight_kg > 0
				? `${sets} · best ${kg(top.weight_kg)} kg × ${time}`
				: `${sets} · best ${time}`;
		}
		return top.weight_kg > 0 ? `${sets} · best ${kg(top.weight_kg)} kg × ${top.reps}` : sets;
	}

	/** Set while saving, so finishing doesn't trigger the redirect below. */
	let saving = false;

	// Opening this page for a workout that is already in history goes back to it.
	$effect(() => {
		if (!saving && workout.current?.finished_at != null) goto(`/workouts/${id}`, { replaceState: true });
	});

	let nothingLogged = $state<Modal>();

	/** No confirmation: the toast offers an undo instead. An empty workout asks first. */
	async function save() {
		// Read the sets now rather than trusting `done`, which may not have loaded yet.
		const current = await loadExercises('workout', id);
		if (!current.some((e) => e.sets.some(logged))) {
			nothingLogged?.open();
			return;
		}
		const finished = id;
		saving = true;
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

	async function saveAsRoutine() {
		const routineId = await saveWorkoutAsRoutine(id);
		await goto(`/routines/${routineId}`);
	}

	async function discard() {
		await deleteWorkout(id);
		await goto('/');
	}
</script>

{#if workout.current}
	{@const w = workout.current}
	<TopBar title="Finish workout" fallback={`/workouts/${id}`}>
		{#snippet action()}
			<button class="primary save" onclick={save}>Save</button>
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
	</div>

	<dl class="stats">
		<div>
			<dt>Exercises</dt>
			<dd>{totals.exercises}</dd>
		</div>
		<div>
			<dt>Sets</dt>
			<dd>{totals.sets}</dd>
		</div>
		<div>
			<dt>Reps</dt>
			<dd>{totals.reps}</dd>
		</div>
		<div>
			<dt>Volume</dt>
			<dd>{kg(totals.volume)} kg</dd>
		</div>
	</dl>

	{#if done.length > 0}
		<ul class="card exercises" aria-label="Exercises done">
			{#each done as e (e.id)}
				<li>
					<span>{e.name}</span>
					<span class="muted">{summary(e)}</span>
				</li>
			{/each}
		</ul>
	{:else if exercises.loaded}
		<p class="muted empty">No sets checked off yet.</p>
	{/if}
	{#if done.length > 0 && unchecked > 0}
		<p class="muted unchecked" data-testid="unchecked">
			{unchecked} unchecked {unchecked === 1 ? 'set' : 'sets'} won't count.
		</p>
	{/if}

	<div class="end-actions">
		<button onclick={saveAsRoutine}>Save as routine</button>
		<ConfirmButton
			label="Discard workout"
			confirmLabel="Discard"
			message="Nothing from this workout will be saved."
			onconfirm={discard}
		/>
	</div>

	<Modal
		bind:this={nothingLogged}
		title="Nothing logged yet"
		message="No sets are checked off. Discard this workout, or keep logging?"
	>
		{#snippet buttons(close)}
			<button
				onclick={async () => {
					close();
					await goto(`/workouts/${id}`);
				}}>Keep logging</button
			>
			<button
				class="destructive"
				onclick={async () => {
					close();
					await discard();
				}}>Discard</button
			>
		{/snippet}
	</Modal>
{:else if workout.loaded}
	<p class="muted">Workout not found. <a href="/profile">Back to history</a></p>
{/if}

<style>
	.save {
		min-height: 32px;
		padding: 0 0.95rem;
		border-radius: 999px;
		font-size: 0.95rem;
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 0.5rem;
		margin: 1rem 0;
	}

	.stats div {
		background: var(--surface);
		border-radius: var(--radius);
		padding: 0.7rem 0.5rem;
		text-align: center;
	}

	.stats dt {
		font-size: 0.75rem;
		color: var(--muted);
	}

	.stats dd {
		margin: 0.2rem 0 0;
		font-size: 1.1rem;
		font-weight: 600;
		white-space: nowrap;
	}

	.exercises {
		list-style: none;
		padding: 0.25rem 0.9rem;
	}

	.exercises li {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		padding: 0.6rem 0;
	}

	.exercises li + li {
		border-top: 1px solid var(--border);
	}

	.end-actions {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
		margin-top: 1.5rem;
	}

	.end-actions > :global(button) {
		width: 100%;
	}

	.empty {
		text-align: center;
		margin: 1.5rem 0;
	}

	.unchecked {
		text-align: center;
		font-size: 0.9rem;
		margin: 0.75rem 0 0;
	}
</style>
