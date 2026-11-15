<script lang="ts">
	import Modal from './Modal.svelte';
	import { exerciseKey } from './exercise-key';
	import {
		durationLabel,
		kg,
		parseDuration,
		parseReps,
		parseWeight,
		repRange,
		repTarget,
		REST_CHOICES,
		restLabel
	} from './format';
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
		setTimed,
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
	/** The open menu opens upward: there was no room for it below the ⋯ button. */
	let menuUp = $state(false);
	/** Exercise whose name is being edited in place. */
	let renaming = $state<string | null>(null);
	/** Exercise waiting for its delete to be confirmed. */
	let deleting = $state<string | null>(null);
	let deleteModal: Modal;
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
		const prev = previous.current.get(exerciseKey(ex.name))?.filter((s) => s.warmup === warm);
		const s = prev?.[nthOfType(ex, i)];
		if (!s) return '–';
		const effort = ex.timed ? durationLabel(s.duration_seconds) : String(s.reps);
		return ex.timed && !s.weight_kg ? effort : `${kg(s.weight_kg)}kg × ${effort}`;
	}

	/** Save a routine set's reps and max reps, and show what was kept (a max at or below the reps is dropped). */
	function saveRepTarget(set: SetView, reps: number, max: number, maxInput: HTMLInputElement) {
		const target = repTarget(reps, max);
		maxInput.value = target.rep_max ? String(target.rep_max) : '';
		updateSet(kind, set.id, target);
	}

	/** Picker options, keeping an off-list value (e.g. synced from elsewhere) selectable. */
	function restOptions(current: number): number[] {
		return REST_CHOICES.includes(current) ? REST_CHOICES : [...REST_CHOICES, current].sort((a, b) => a - b);
	}

	function setType(set: SetView, warmup: boolean) {
		typeFor = null;
		if (set.warmup !== warmup) updateSet(kind, set.id, { warmup });
	}

	/** Run a menu action and close the menu. */
	function pick(action: () => unknown) {
		menuFor = null;
		void action();
	}

	function saveName(ex: ExerciseView, value: string) {
		const v = value.trim();
		if (renaming !== ex.id) return;
		renaming = null;
		if (v && v !== ex.name) renameExercise(kind, ex.id, v);
	}

	/**
	 * The menu opens over its ⋯ button, growing down from it. Near the bottom of the screen
	 * it grows up instead, and if neither way fits it is scrolled into view.
	 */
	function placeMenu(menu: HTMLElement) {
		const margin = 8;
		// offsetHeight, not the bounding box: the grow-in animation starts scaled down.
		const height = menu.offsetHeight;
		const button = menu.parentElement!.getBoundingClientRect();
		if (button.top + height <= innerHeight - margin) return;
		if (button.bottom - height >= margin) menuUp = true;
		else menu.scrollIntoView({ block: 'nearest' });
	}

	/** Focuses and selects the name field as it appears. */
	function focusSelect(el: HTMLInputElement) {
		el.focus();
		el.select();
	}

	/** Workout sets are checked off as they are performed; only checked sets count. */
	const checkable = $derived(kind === 'workout');
</script>

<svelte:window
	onclick={(e) => {
		const target = e.target as Element;
		if (typeFor && !target.closest('.set-cell')) typeFor = null;
		if (menuFor && !target.closest('.ex-menu')) menuFor = null;
	}}
	onkeydown={(e) => {
		if (e.key === 'Escape') menuFor = typeFor = null;
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
					{#if renaming === ex.id}
						<input
							class="rename"
							aria-label="Exercise name"
							value={ex.name}
							{@attach focusSelect}
							onblur={(e) => saveName(ex, e.currentTarget.value)}
							onkeydown={(e) => {
								if (e.key === 'Enter') e.currentTarget.blur();
								if (e.key === 'Escape') renaming = null;
							}}
						/>
					{:else}
						<h2>{ex.name}</h2>
					{/if}
					<div class="ex-menu">
						<button
							class="menu-button"
							aria-label="Exercise options"
							aria-haspopup="menu"
							aria-expanded={menuFor === ex.id}
							onclick={() => {
								menuUp = false;
								menuFor = menuFor === ex.id ? null : ex.id;
							}}
						>
							<svg viewBox="0 0 24 24" aria-hidden="true">
								<circle cx="12" cy="5" r="2" /><circle cx="12" cy="12" r="2" /><circle
									cx="12"
									cy="19"
									r="2"
								/>
							</svg>
						</button>
						{#if menuFor === ex.id}
							<div
								class="dropdown"
								class:up={menuUp}
								role="menu"
								aria-label="{ex.name} options"
								{@attach placeMenu}
							>
								<button role="menuitem" onclick={() => pick(() => (renaming = ex.id))}>
									<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16z" /></svg>
									Rename
								</button>
								<!-- A per-exercise setting: it applies to this exercise in every workout and routine. -->
								<button role="menuitem" onclick={() => pick(() => setTimed(ex.name, !ex.timed))}>
									{#if ex.timed}
										<svg viewBox="0 0 24 24" aria-hidden="true"
											><path d="M9 6h11M9 12h11M9 18h11M4.5 6h.01M4.5 12h.01M4.5 18h.01" /></svg
										>
										Log reps
									{:else}
										<svg viewBox="0 0 24 24" aria-hidden="true"
											><circle cx="12" cy="13" r="8" /><path d="M12 9v4l2.5 2.5M10 2.5h4" /></svg
										>
										Log time
									{/if}
								</button>
								{#if block.superset}
									<button
										role="menuitem"
										aria-label="Move {ex.name} up in superset"
										disabled={ei === 0}
										onclick={() => pick(() => moveWithinSuperset(kind, parentId, ex.id, -1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 19V5M6 11l6-6 6 6" /></svg>
										Up in superset
									</button>
									<button
										role="menuitem"
										aria-label="Move {ex.name} down in superset"
										disabled={ei === block.items.length - 1}
										onclick={() => pick(() => moveWithinSuperset(kind, parentId, ex.id, 1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M6 13l6 6 6-6" /></svg>
										Down in superset
									</button>
									<button
										role="menuitem"
										aria-label="Move superset up"
										disabled={bi === 0}
										onclick={() => pick(() => moveBlock(kind, parentId, ex.id, -1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 19V5M6 11l6-6 6 6" /></svg>
										Move superset up
									</button>
									<button
										role="menuitem"
										aria-label="Move superset down"
										disabled={bi === blocks.length - 1}
										onclick={() => pick(() => moveBlock(kind, parentId, ex.id, 1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M6 13l6 6 6-6" /></svg>
										Move superset down
									</button>
								{:else}
									<button
										role="menuitem"
										aria-label="Move {ex.name} up"
										disabled={bi === 0}
										onclick={() => pick(() => moveBlock(kind, parentId, ex.id, -1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 19V5M6 11l6-6 6 6" /></svg>
										Move up
									</button>
									<button
										role="menuitem"
										aria-label="Move {ex.name} down"
										disabled={bi === blocks.length - 1}
										onclick={() => pick(() => moveBlock(kind, parentId, ex.id, 1))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M6 13l6 6 6-6" /></svg>
										Move down
									</button>
								{/if}
								{#if ei === 0 && bi > 0}
									<button role="menuitem" onclick={() => pick(() => linkToAbove(kind, parentId, ex.id))}>
										<svg viewBox="0 0 24 24" aria-hidden="true"
											><path
												d="M10 14a4 4 0 0 0 5.66 0l3-3a4 4 0 0 0-5.66-5.66l-1 1M14 10a4 4 0 0 0-5.66 0l-3 3a4 4 0 0 0 5.66 5.66l1-1"
											/></svg
										>
										Superset with above
									</button>
								{/if}
								{#if block.superset}
									<button role="menuitem" onclick={() => pick(() => unlink(kind, parentId, ex.id))}>
										<svg viewBox="0 0 24 24" aria-hidden="true"
											><path d="M15 9l-6 6M9 9l6 6" /><circle cx="12" cy="12" r="9" /></svg
										>
										Remove from superset
									</button>
								{/if}
								<button
									role="menuitem"
									class="danger-item"
									onclick={() =>
										pick(() => {
											deleting = ex.id;
											deleteModal.open();
										})}
								>
									<svg viewBox="0 0 24 24" aria-hidden="true"
										><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" /></svg
									>
									Delete exercise
								</button>
							</div>
						{/if}
					</div>
				</header>

				<div class="sets" class:with-prev={kind === 'workout'}>
					<span class="th">Set</span>
					{#if kind === 'workout'}<span class="th">Previous</span>{/if}
					<span class="th">
						<svg class="kg-icon" viewBox="0 0 24 24" aria-hidden="true">
							<path d="M2 12h20M6 8v8M18 8v8M9 9.5v5M15 9.5v5" />
						</svg>
						kg
					</span>
					<span class="th">{ex.timed ? 'Time' : 'Reps'}</span>
					<span class="th"></span>

					{#each ex.sets as set, si (set.id)}
						{@const done = checkable && set.done}
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
									<button role="menuitem" onclick={() => setType(set, false)}>
										<span class="tag">1</span> Normal set
									</button>
									<button role="menuitem" onclick={() => setType(set, true)}>
										<span class="tag warmup">W</span> Warm-up set
									</button>
									{#if checkable}
										<button
											role="menuitem"
											class="danger-item"
											aria-label="Delete set {si + 1}"
											onclick={() => {
												typeFor = null;
												deleteSet(kind, set.id);
											}}
										>
											<span class="tag">×</span> Delete set
										</button>
									{/if}
								</span>
							{/if}
						</span>
						{#if kind === 'workout'}
							<span class="prev" class:done data-testid="previous">{prevFor(ex, si)}</span>
						{/if}
						<input
							class:done
							inputmode="decimal"
							aria-label="Weight set {si + 1}"
							placeholder="0"
							value={set.weight_kg ? kg(set.weight_kg) : ''}
							onchange={(e) => updateSet(kind, set.id, { weight_kg: parseWeight(e.currentTarget.value) })}
						/>
						{#if ex.timed}
							<!-- Seconds or m:ss; the decimal keypad's separator works as the colon. -->
							<input
								class:done
								inputmode="decimal"
								aria-label="Time set {si + 1}"
								placeholder="0:00"
								value={set.duration_seconds ? durationLabel(set.duration_seconds) : ''}
								onchange={(e) => {
									const duration_seconds = parseDuration(e.currentTarget.value);
									e.currentTarget.value = duration_seconds ? durationLabel(duration_seconds) : '';
									updateSet(kind, set.id, { duration_seconds });
								}}
							/>
						{:else if checkable}
							<!-- The rep range, when the routine set one, sits under the reps done. -->
							<span class="reps-cell" class:ranged={set.rep_max > 0}>
								<input
									class:done
									inputmode="numeric"
									pattern="[0-9]*"
									aria-label="Reps set {si + 1}"
									placeholder={set.rep_max > 0 ? repRange(set) : '0'}
									value={set.reps ? String(set.reps) : ''}
									onchange={(e) => updateSet(kind, set.id, { reps: parseReps(e.currentTarget.value) })}
								/>
								{#if set.rep_max > 0 && set.reps}
									<span class="target" data-testid="rep-range">{repRange(set)}</span>
								{/if}
							</span>
						{:else}
							<!-- Reps, and optionally a max that makes them a range (8 and 12 plan 8–12). -->
							<span class="rep-range">
								<input
									inputmode="numeric"
									pattern="[0-9]*"
									aria-label="Reps set {si + 1}"
									placeholder="0"
									value={set.reps ? String(set.reps) : ''}
									onchange={(e) => {
										const max = e.currentTarget.parentElement!.querySelector<HTMLInputElement>('.max')!;
										saveRepTarget(set, parseReps(e.currentTarget.value), set.rep_max, max);
									}}
								/>
								<span class="dash" aria-hidden="true">–</span>
								<input
									class="max"
									inputmode="numeric"
									pattern="[0-9]*"
									aria-label="Maximum reps for set {si + 1}"
									placeholder="0"
									value={set.rep_max ? String(set.rep_max) : ''}
									onchange={(e) =>
										saveRepTarget(set, set.reps, parseReps(e.currentTarget.value), e.currentTarget)}
								/>
							</span>
						{/if}
						{#if checkable}
							<button
								class="check-set"
								class:done
								aria-label="Complete set {si + 1}"
								aria-pressed={set.done}
								onclick={() => updateSet(kind, set.id, { done: !set.done })}
							>
								<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
							</button>
						{:else}
							<button
								class="delete-set"
								aria-label="Delete set {si + 1}"
								onclick={() => deleteSet(kind, set.id)}
							>
								<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 7l10 10M17 7L7 17" /></svg>
							</button>
						{/if}
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
									onchange={(e) => updateSet(kind, set.id, { rest_seconds: Number(e.currentTarget.value) })}
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

<!-- One confirm for every exercise; the menu that asked has closed by the time it shows. -->
<Modal bind:this={deleteModal} title="Delete exercise?" message="This can't be undone.">
	{#snippet buttons(close)}
		<button onclick={close}>Cancel</button>
		<button
			class="destructive"
			onclick={async () => {
				close();
				if (deleting) await deleteExercise(kind, parentId, deleting);
				deleting = null;
			}}>Delete</button
		>
	{/snippet}
</Modal>
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

	.ex-menu {
		position: relative;
		flex: none;
	}

	.rename {
		flex: 1;
		min-width: 0;
		font-size: 1.3rem;
		font-weight: 600;
		color: var(--accent);
		background: transparent;
		padding: 0.15rem 0.4rem;
	}

	/* Options for one exercise, under its ⋯ button. Same look as the set type menu. */
	/* Opens over the ⋯ button, which it hides, and grows out of its corner. */
	.dropdown {
		position: absolute;
		top: 0;
		right: 0;
		z-index: 20;
		transform-origin: top right;
		animation: menu-in 0.16s cubic-bezier(0.2, 0.9, 0.3, 1.1);
		display: flex;
		flex-direction: column;
		min-width: 220px;
		padding: 4px;
		border-radius: 12px;
		background: var(--surface);
		box-shadow:
			0 12px 32px rgba(0, 0, 0, 0.35),
			0 0 0 1px var(--border);
	}

	.dropdown.up {
		top: auto;
		bottom: 0;
		transform-origin: bottom right;
	}

	@keyframes menu-in {
		from {
			opacity: 0;
			transform: scale(0.85);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.dropdown {
			animation: none;
		}
	}

	.dropdown button {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		min-height: 44px;
		padding: 0 0.7rem;
		background: transparent;
		border-radius: 8px;
		text-align: left;
		font-size: 0.95rem;
		white-space: nowrap;
	}

	.dropdown button:active:not(:disabled) {
		background: var(--border);
	}

	.dropdown svg {
		width: 18px;
		height: 18px;
		flex: none;
	}

	/* Delete sits apart at the bottom. */
	.dropdown .danger-item {
		margin-top: 4px;
		border-top: 1px solid var(--border);
		border-radius: 0 0 8px 8px;
	}

	.menu-button {
		background: transparent;
		padding: 0;
		min-width: 40px;
		color: var(--text);
	}

	.menu-button[aria-expanded='true'] {
		visibility: hidden;
	}

	.menu-button svg {
		width: 22px;
		height: 22px;
		fill: currentColor;
		stroke: none;
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

	.check-set {
		display: grid;
		place-items: center;
		height: 44px;
		padding: 0;
		border-radius: 10px;
		background: var(--surface);
		color: var(--muted);
	}

	.check-set.done {
		background: var(--accent);
		color: var(--accent-text);
	}

	.check-set svg {
		width: 22px;
		height: 22px;
		stroke-width: 3;
	}

	/* A checked-off row is tinted so what's left to do stands out. */
	.sets input.done {
		background: color-mix(in srgb, var(--accent) 16%, transparent);
		border-color: transparent;
	}

	.prev.done {
		color: var(--text);
	}

	.reps-cell {
		position: relative;
		display: block;
	}

	/* Routine reps: two boxes, reps and an optional max, sharing the column. */
	.rep-range {
		display: flex;
		align-items: center;
		gap: 0.2rem;
		min-width: 0;
	}

	.rep-range input {
		flex: 1;
		min-width: 0;
	}

	.dash {
		color: var(--muted);
	}

	/* Room under the number for the range. */
	.reps-cell.ranged input {
		padding-bottom: 0.7rem;
	}

	.target {
		position: absolute;
		left: 0;
		right: 0;
		bottom: 3px;
		text-align: center;
		font-size: 0.68rem;
		font-weight: 600;
		color: var(--muted);
		pointer-events: none;
		font-variant-numeric: tabular-nums;
	}

	.danger-item {
		color: var(--danger);
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
