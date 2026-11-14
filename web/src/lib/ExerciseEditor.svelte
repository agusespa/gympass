<script lang="ts">
	import ConfirmButton from './ConfirmButton.svelte';
	import { kg, parseReps, parseWeight, REST_CHOICES, restLabel } from './format';
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
	/** Exercise whose options menu is open; one at a time. */
	let menuFor = $state<string | null>(null);
	/** Set whose type picker (normal / warm-up) is open. */
	let typeFor = $state<string | null>(null);
	let newName = $state('');

	async function add(e: SubmitEvent) {
		e.preventDefault();
		const name = newName.trim();
		if (!name) return;
		newName = '';
		await addExercise(kind, parentId, name);
	}

	/** Position of a set among the exercise's sets of the same type (warm-up or working), 0-based. */
	function nthOfType(ex: ExerciseView, i: number): number {
		const warm = ex.sets[i].warmup;
		return ex.sets.slice(0, i).filter((s) => s.warmup === warm).length;
	}

	/** Last time's set of the same type and order: warm-ups match warm-ups, working sets match working sets. */
	function prevFor(ex: ExerciseView, i: number): string {
		const warm = ex.sets[i].warmup;
		const prev = previous.current.get(ex.name.trim().toLowerCase())?.filter((s) => s.warmup === warm);
		const s = prev?.[nthOfType(ex, i)];
		return s ? `${kg(s.weight_kg)}kg × ${s.reps}` : '–';
	}

	/** Picker options, keeping an off-list value (e.g. synced from elsewhere) selectable. */
	function restOptions(current: number): number[] {
		return REST_CHOICES.includes(current)
			? REST_CHOICES
			: [...REST_CHOICES, current].sort((a, b) => a - b);
	}

	function setType(kind: Kind, set: SetView, warmup: boolean) {
		typeFor = null;
		if (set.warmup !== warmup) updateSet(kind, set.id, { warmup });
	}
</script>

<svelte:window
	onclick={(e) => {
		if (typeFor && !(e.target as Element).closest('.set-cell')) typeFor = null;
	}}
/>

{#each blocks as block, bi (block.items[0].id)}
	<div class:superset={block.superset} class="block">
		{#if block.superset}
			<span class="badge">Superset</span>
		{/if}

		{#each block.items as ex, ei (ex.id)}
			<section aria-label={ex.name}>
				<header>
					<h2>{ex.name}</h2>
					<button
						class="menu-button"
						aria-label="Exercise options"
						aria-expanded={menuFor === ex.id}
						onclick={() => (menuFor = menuFor === ex.id ? null : ex.id)}
					>
						<svg viewBox="0 0 24 24" aria-hidden="true">
							<circle cx="12" cy="5" r="2" /><circle cx="12" cy="12" r="2" /><circle
								cx="12"
								cy="19"
								r="2"
							/>
						</svg>
					</button>
				</header>

				{#if menuFor === ex.id}
					<div class="menu">
						<input
							aria-label="Exercise name"
							value={ex.name}
							onchange={(e) => {
								const v = e.currentTarget.value.trim();
								if (v) renameExercise(kind, ex.id, v);
							}}
						/>
						<div class="chips">
							{#if block.superset}
								<button
									class="chip"
									aria-label="Move {ex.name} up in superset"
									disabled={ei === 0}
									onclick={() => moveWithinSuperset(kind, parentId, ex.id, -1)}>↑ In superset</button
								>
								<button
									class="chip"
									aria-label="Move {ex.name} down in superset"
									disabled={ei === block.items.length - 1}
									onclick={() => moveWithinSuperset(kind, parentId, ex.id, 1)}>↓ In superset</button
								>
								<button
									class="chip"
									aria-label="Move superset up"
									disabled={bi === 0}
									onclick={() => moveBlock(kind, parentId, ex.id, -1)}>↑ Superset</button
								>
								<button
									class="chip"
									aria-label="Move superset down"
									disabled={bi === blocks.length - 1}
									onclick={() => moveBlock(kind, parentId, ex.id, 1)}>↓ Superset</button
								>
							{:else}
								<button
									class="chip"
									aria-label="Move {ex.name} up"
									disabled={bi === 0}
									onclick={() => moveBlock(kind, parentId, ex.id, -1)}>↑ Move up</button
								>
								<button
									class="chip"
									aria-label="Move {ex.name} down"
									disabled={bi === blocks.length - 1}
									onclick={() => moveBlock(kind, parentId, ex.id, 1)}>↓ Move down</button
								>
							{/if}
							{#if ei === 0 && bi > 0}
								<button class="chip" onclick={() => linkToAbove(kind, parentId, ex.id)}>
									Superset with above
								</button>
							{/if}
							{#if block.superset}
								<button class="chip" onclick={() => unlink(kind, parentId, ex.id)}>
									Remove from superset
								</button>
							{/if}
						</div>
						<ConfirmButton
							label="Delete exercise"
							onconfirm={() => deleteExercise(kind, parentId, ex.id)}
						/>
					</div>
				{/if}

				<div class="sets" class:with-prev={kind === 'workout'}>
					<span class="th">Set</span>
					{#if kind === 'workout'}<span class="th">Previous</span>{/if}
					<span class="th">
						<svg class="kg-icon" viewBox="0 0 24 24" aria-hidden="true">
							<path d="M2 12h20M6 8v8M18 8v8M9 9.5v5M15 9.5v5" />
						</svg>
						kg
					</span>
					<span class="th">Reps</span>
					<span class="th"></span>

					{#each ex.sets as set, si (set.id)}
						<span class="set-cell">
							<button
								class="set-num"
								class:warmup={set.warmup}
								aria-label="Set {si + 1} type: {set.warmup ? 'warm-up' : 'normal'}"
								aria-expanded={typeFor === set.id}
								onclick={() => (typeFor = typeFor === set.id ? null : set.id)}
								>{set.warmup ? 'W' : nthOfType(ex, si) + 1}</button
							>
							{#if typeFor === set.id}
								<span class="type-menu" role="menu">
									<button role="menuitem" onclick={() => setType(kind, set, false)}>
										<span class="tag">1</span> Normal set
									</button>
									<button role="menuitem" onclick={() => setType(kind, set, true)}>
										<span class="tag warmup">W</span> Warm-up set
									</button>
								</span>
							{/if}
						</span>
						{#if kind === 'workout'}
							<span class="prev" data-testid="previous">{prevFor(ex, si)}</span>
						{/if}
						<input
							inputmode="decimal"
							aria-label="Weight set {si + 1}"
							placeholder="0"
							value={set.weight_kg ? kg(set.weight_kg) : ''}
							onchange={(e) => updateSet(kind, set.id, { weight_kg: parseWeight(e.currentTarget.value) })}
						/>
						<input
							inputmode="numeric"
							pattern="[0-9]*"
							aria-label="Reps set {si + 1}"
							placeholder="0"
							value={set.reps ? String(set.reps) : ''}
							onchange={(e) => updateSet(kind, set.id, { reps: parseReps(e.currentTarget.value) })}
						/>
						<button
							class="delete-set"
							aria-label="Delete set {si + 1}"
							onclick={() => deleteSet(kind, set.id)}
						>
							<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 7l10 10M17 7L7 17" /></svg>
						</button>
						<!-- In a superset you only rest after the last exercise of the round. -->
						{#if !block.superset || ei === block.items.length - 1}
							<label class="rest" class:warmup={set.warmup} class:none={set.rest_seconds === 0}>
								<span class="rest-pill">
									<svg viewBox="0 0 24 24" aria-hidden="true">
										<circle cx="12" cy="13" r="8" /><path d="M12 9v4l2.5 2.5M10 2.5h4" />
									</svg>
									{restLabel(set.rest_seconds)}
								</span>
								<select
									aria-label="Rest after set {si + 1}"
									value={set.rest_seconds}
									onchange={(e) =>
										updateSet(kind, set.id, { rest_seconds: Number(e.currentTarget.value) })}
								>
									{#each restOptions(set.rest_seconds) as sec (sec)}
										<option value={sec}>{sec === 0 ? 'No rest' : restLabel(sec)}</option>
									{/each}
								</select>
							</label>
						{/if}
					{/each}
				</div>

				<button class="add-set" onclick={() => addSet(kind, ex.id)}>
					<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
					Add set
				</button>
			</section>
		{/each}
	</div>
{/each}

<form class="add-exercise" onsubmit={add}>
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
	section {
		padding: 1.25rem 0 1.5rem;
		border-bottom: 1px solid var(--border);
	}

	.block.superset {
		border-left: 3px solid var(--superset);
		padding-left: 0.75rem;
		margin-top: 0.75rem;
	}

	.badge {
		display: block;
		padding-top: 0.25rem;
		color: var(--superset);
		font-weight: 700;
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
	}

	h2 {
		flex: 1;
		min-width: 0;
		color: var(--accent);
		font-size: 1.3rem;
		font-weight: 600;
	}

	svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.menu-button {
		background: transparent;
		padding: 0;
		min-width: 40px;
		color: var(--text);
	}

	.menu-button svg {
		width: 22px;
		height: 22px;
		fill: currentColor;
		stroke: none;
	}

	.menu {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.6rem;
		background: var(--surface);
		border-radius: var(--radius);
		padding: 0.75rem;
		margin-bottom: 1rem;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.chip {
		background: var(--border);
		border-radius: 999px;
		min-height: 36px;
		padding: 0.35rem 0.85rem;
		font-size: 0.9rem;
	}

	/* Set table as a grid: Set | (Previous) | kg | Reps | delete */
	.sets {
		display: grid;
		grid-template-columns: 44px 1fr 1fr 44px;
		align-items: center;
		gap: 0.6rem 0.6rem;
	}

	.sets.with-prev {
		grid-template-columns: 44px 1.15fr 1fr 1fr 44px;
	}

	.th {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.3rem;
		font-size: 0.8rem;
		font-weight: 500;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted);
	}

	.kg-icon {
		width: 16px;
		height: 16px;
		stroke-width: 2.5;
	}

	.set-cell {
		position: relative;
	}

	.set-num {
		display: grid;
		place-items: center;
		width: 100%;
		height: 44px;
		min-height: 0;
		padding: 0;
		border-radius: 10px;
		background: var(--surface);
		font-weight: 600;
		font-size: 1.05rem;
	}

	.set-num.warmup,
	.tag.warmup {
		color: var(--warmup);
	}

	.type-menu {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		z-index: 20;
		display: flex;
		flex-direction: column;
		min-width: 180px;
		padding: 4px;
		border-radius: 12px;
		background: var(--surface);
		box-shadow:
			0 12px 32px rgba(0, 0, 0, 0.35),
			0 0 0 1px var(--border);
	}

	.type-menu button {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-height: 40px;
		padding: 0 0.6rem;
		background: transparent;
		border-radius: 8px;
		text-align: left;
		font-size: 0.95rem;
	}

	.type-menu button:active {
		background: var(--border);
	}

	.tag {
		width: 1.2rem;
		text-align: center;
		font-weight: 700;
	}

	.prev {
		text-align: center;
		color: var(--muted);
		white-space: nowrap;
		font-size: 0.95rem;
	}

	.sets input {
		height: 44px;
		text-align: center;
		font-size: 1.05rem;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0 0.25rem;
	}

	.delete-set {
		display: grid;
		place-items: center;
		height: 44px;
		padding: 0;
		border-radius: 10px;
		background: var(--surface);
		color: var(--muted);
	}

	.delete-set svg,
	.add-set svg {
		width: 20px;
		height: 20px;
	}

	/* Rest line between sets: a centred pill on a hairline; a transparent native
	   select on top gives the iOS picker wheel when tapped. */
	.rest {
		grid-column: 1 / -1;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		height: 22px;
		margin: -0.2rem 0;
	}

	.rest::before {
		content: '';
		position: absolute;
		left: 52px;
		right: 52px;
		top: 50%;
		border-top: 1px dashed var(--border);
	}

	.rest-pill {
		position: relative;
		display: flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.1rem 0.55rem;
		border-radius: 999px;
		background: var(--bg);
		color: var(--accent);
		font-size: 0.78rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}

	.rest.warmup .rest-pill {
		color: var(--warmup);
	}

	.rest.none .rest-pill {
		color: var(--muted);
	}

	.rest-pill svg {
		width: 13px;
		height: 13px;
		stroke-width: 2.2;
	}

	.rest select {
		position: absolute;
		inset: 0;
		width: 100%;
		opacity: 0;
		font-size: 16px;
	}

	.add-set {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		width: 100%;
		min-height: 48px;
		margin-top: 1rem;
		background: var(--surface);
		border-radius: 12px;
		font-size: 1.05rem;
		font-weight: 500;
	}

	.add-exercise {
		display: flex;
		gap: 0.5rem;
		margin-top: 1.25rem;
	}

	.add-exercise input {
		height: 48px;
		background: var(--surface);
		border-color: transparent;
		border-radius: 12px;
		padding: 0 1rem;
	}

	.add-exercise button {
		min-height: 48px;
		border-radius: 12px;
		padding: 0 1.25rem;
	}
</style>
