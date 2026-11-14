<script lang="ts">
	import { kg, parseReps, parseWeight } from './format';
	import { live } from './live.svelte';
	import {
		addExercise,
		addSet,
		deleteExercise,
		deleteSet,
		exerciseNames,
		linkToAbove,
		loadExercises,
		moveBlock,
		moveWithinSuperset,
		previousSets,
		renameExercise,
		toBlocks,
		unlink,
		updateSet,
		type ExerciseView,
		type Kind,
		type SetView
	} from './repo';

	let { kind, parentId }: { kind: Kind; parentId: string } = $props();

	const exercises = live(
		() => [kind, parentId] as const,
		([k, id]) => loadExercises(k, id),
		[] as ExerciseView[]
	);
	const previous = live(
		() => [kind, parentId] as const,
		([k, id]) => (k === 'workout' ? previousSets(id) : Promise.resolve(new Map<string, SetView[]>())),
		new Map<string, SetView[]>()
	);
	const names = live(
		() => null,
		() => exerciseNames(),
		[] as string[]
	);

	const blocks = $derived(toBlocks(exercises.current));
	let editing = $state(false);
	let newName = $state('');

	async function add(e: SubmitEvent) {
		e.preventDefault();
		const name = newName.trim();
		if (!name) return;
		newName = '';
		await addExercise(kind, parentId, name);
	}

	function prevFor(ex: ExerciseView, i: number): string {
		const s = previous.current.get(ex.name.trim().toLowerCase())?.[i];
		return s ? `${s.reps} × ${kg(s.weight_kg)}` : '–';
	}

	async function remove(ex: ExerciseView) {
		if (confirm(`Delete ${ex.name}?`)) await deleteExercise(kind, parentId, ex.id);
	}
</script>

{#if exercises.current.length > 0}
	<div class="row toolbar">
		<span class="spacer"></span>
		<button class="ghost" onclick={() => (editing = !editing)}>
			{editing ? 'Done' : 'Edit order'}
		</button>
	</div>
{/if}

{#each blocks as block, bi (block.items[0].id)}
	<div class:superset={block.superset} class="block">
		{#if block.superset}
			<div class="row superset-head">
				<span class="badge">Superset</span>
				<span class="spacer"></span>
				{#if editing}
					<button
						class="icon"
						aria-label="Move superset up"
						disabled={bi === 0}
						onclick={() => moveBlock(kind, parentId, block.items[0].id, -1)}>↑</button
					>
					<button
						class="icon"
						aria-label="Move superset down"
						disabled={bi === blocks.length - 1}
						onclick={() => moveBlock(kind, parentId, block.items[0].id, 1)}>↓</button
					>
				{/if}
			</div>
		{/if}

		{#each block.items as ex, ei (ex.id)}
			<section class="card" aria-label={ex.name}>
				{#if editing}
					<input
						class="name-input"
						aria-label="Exercise name"
						value={ex.name}
						onchange={(e) => {
							const v = e.currentTarget.value.trim();
							if (v) renameExercise(kind, ex.id, v);
						}}
					/>
					<div class="row controls">
						{#if block.superset}
							<button
								class="icon"
								aria-label="Move {ex.name} up in superset"
								disabled={ei === 0}
								onclick={() => moveWithinSuperset(kind, parentId, ex.id, -1)}>↑</button
							>
							<button
								class="icon"
								aria-label="Move {ex.name} down in superset"
								disabled={ei === block.items.length - 1}
								onclick={() => moveWithinSuperset(kind, parentId, ex.id, 1)}>↓</button
							>
						{:else}
							<button
								class="icon"
								aria-label="Move {ex.name} up"
								disabled={bi === 0}
								onclick={() => moveBlock(kind, parentId, ex.id, -1)}>↑</button
							>
							<button
								class="icon"
								aria-label="Move {ex.name} down"
								disabled={bi === blocks.length - 1}
								onclick={() => moveBlock(kind, parentId, ex.id, 1)}>↓</button
							>
						{/if}
						{#if ei === 0 && bi > 0}
							<button class="ghost" onclick={() => linkToAbove(kind, parentId, ex.id)}>
								Superset with above
							</button>
						{/if}
						{#if block.superset}
							<button class="ghost" onclick={() => unlink(kind, parentId, ex.id)}>Remove from superset</button>
						{/if}
						<span class="spacer"></span>
						<button class="danger" onclick={() => remove(ex)}>Delete</button>
					</div>
				{:else}
					<h2>{ex.name}</h2>
					<table>
						<thead>
							<tr>
								<th class="num">Set</th>
								{#if kind === 'workout'}<th>Previous</th>{/if}
								<th>kg</th>
								<th>Reps</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each ex.sets as set, si (set.id)}
								<tr>
									<td class="num muted">{si + 1}</td>
									{#if kind === 'workout'}
										<td class="prev muted" data-testid="previous">{prevFor(ex, si)}</td>
									{/if}
									<td>
										<input
											inputmode="decimal"
											aria-label="Weight set {si + 1}"
											placeholder="0"
											value={set.weight_kg ? kg(set.weight_kg) : ''}
											onchange={(e) =>
												updateSet(kind, set.id, { weight_kg: parseWeight(e.currentTarget.value) })}
										/>
									</td>
									<td>
										<input
											inputmode="numeric"
											pattern="[0-9]*"
											aria-label="Reps set {si + 1}"
											placeholder="0"
											value={set.reps ? String(set.reps) : ''}
											onchange={(e) =>
												updateSet(kind, set.id, { reps: parseReps(e.currentTarget.value) })}
										/>
									</td>
									<td>
										<button
											class="icon danger"
											aria-label="Delete set {si + 1}"
											onclick={() => deleteSet(kind, set.id)}>×</button
										>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					<button class="add-set" onclick={() => addSet(kind, ex.id)}>+ Add set</button>
				{/if}
			</section>
		{/each}
	</div>
{/each}

<form class="card row" onsubmit={add}>
	<input
		list="exercise-names"
		aria-label="New exercise name"
		placeholder="Add exercise"
		autocomplete="off"
		bind:value={newName}
	/>
	<button class="primary" type="submit" disabled={!newName.trim()}>Add</button>
</form>
<datalist id="exercise-names">
	{#each names.current as n (n)}
		<option value={n}></option>
	{/each}
</datalist>

<style>
	.toolbar {
		margin-bottom: 0.25rem;
	}

	.block.superset {
		border-left: 3px solid var(--superset);
		padding-left: 0.5rem;
		margin-bottom: 0.75rem;
	}

	.block.superset .card {
		margin-bottom: 0.5rem;
	}

	.superset-head {
		margin-bottom: 0.4rem;
	}

	.badge {
		color: var(--superset);
		font-weight: 700;
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		margin: 0.5rem 0;
	}

	th {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--muted);
		text-align: left;
		padding: 0 0.25rem 0.25rem;
	}

	td {
		padding: 0.2rem 0.25rem;
	}

	.num {
		width: 2rem;
		text-align: center;
	}

	.prev {
		font-size: 0.9rem;
		white-space: nowrap;
	}

	td input {
		text-align: center;
	}

	td:last-child,
	th:last-child {
		width: 40px;
	}

	.add-set {
		width: 100%;
	}

	.name-input {
		font-weight: 600;
		margin-bottom: 0.5rem;
	}

	.controls {
		flex-wrap: wrap;
		gap: 0.25rem;
	}
</style>
