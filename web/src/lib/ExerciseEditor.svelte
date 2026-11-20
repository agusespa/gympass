<script lang="ts">
	import { tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import { backOut } from 'svelte/easing';
	import { fade, scale, slide, type SlideParams } from 'svelte/transition';
	import { motion } from './motion';
	import { CATALOG } from './catalog';
	import EquipmentIcon from './EquipmentIcon.svelte';
	import Modal from './Modal.svelte';
	import type { Progression } from './progression';
	import {
		durationLabel,
		kg,
		parseDuration,
		parseReps,
		parseWeight,
		plural,
		repRange,
		repTarget,
		REST_CHOICES
	} from './format';
	import { live } from './live.svelte';
	import {
		addExercise,
		addSet,
		applyProgression,
		deleteExercise,
		deleteSet,
		linkToAbove,
		loadExercises,
		loadProgressions,
		moveBlock,
		moveWithinSuperset,
		toBlocks,
		unlink,
		updateSet,
		type Block,
		type ExerciseView,
		type Kind,
		type SetView
	} from './repo';

	let {
		kind,
		parentId,
		collapseDone = false
	}: {
		kind: Kind;
		parentId: string;
		/** Fold an exercise down to its name once every set is checked off (a running workout). */
		collapseDone?: boolean;
	} = $props();

	const exercises = live(
		() => [kind, parentId] as const,
		([k, id]) => loadExercises(k, id),
		[] as ExerciseView[]
	);
	/** What adding a step of weight would do, by exercise entry id; only a running workout has any. */
	const progressions = live(
		() => [kind, parentId] as const,
		async ([k, id]): Promise<Record<string, Progression>> => (k === 'workout' ? loadProgressions(id) : {}),
		{}
	);

	const blocks = $derived(toBlocks(exercises.current));

	/** Superset colours (app.css), cycled so neighbouring supersets look different. */
	const SUPERSET_COLORS = ['--superset', '--superset-2', '--superset-3'];
	/** Each block's superset colour, or undefined for a single exercise. */
	const supersetColor = $derived.by(() => {
		let n = 0;
		return blocks.map((b) =>
			b.superset ? `var(${SUPERSET_COLORS[n++ % SUPERSET_COLORS.length]})` : undefined
		);
	});

	/** Finished exercises the user opened again. */
	let expanded = $state<Record<string, boolean>>({});

	// Folding an exercise slides its sets away instead of cutting them.
	const foldMs = motion(250);

	// Sets and exercises slide in and out as they are added and deleted, but not while the
	// screen first fills in: until then everything just appears.
	let settled = false;
	$effect(() => {
		if (exercises.loaded && !settled) void tick().then(() => (settled = true));
	});
	const rowMs = () => (settled ? motion(220) : 0);
	/** Slides a deleted row away. While it goes it is only a picture: it can't be tapped or read out. */
	function slideOut(node: HTMLElement, params: SlideParams) {
		node.inert = true;
		node.setAttribute('aria-hidden', 'true');
		return slide(node, params);
	}

	function allDone(ex: ExerciseView): boolean {
		return collapseDone && ex.sets.length > 0 && ex.sets.every((s) => s.done);
	}
	/** Exercise whose options menu is open; one at a time. */
	let menuFor = $state<string | null>(null);
	/** The open menu opens upward: there was no room for it below the ⋯ button. */
	let menuUp = $state(false);
	/** Exercise waiting for its delete to be confirmed. */
	let deleting = $state<string | null>(null);
	let deleteModal: Modal;
	/** Set whose type picker (normal / warm-up) is open. */
	let typeFor = $state<string | null>(null);
	/** The add box is in use: the catalogue is listed, narrowed by what is typed in it. */
	let picking = $state(false);
	let query = $state('');
	const matches = $derived.by(() => {
		if (!picking) return [];
		const q = query.trim().toLowerCase();
		return CATALOG.filter((c) => c.name.toLowerCase().includes(q)).sort((a, b) =>
			a.name.localeCompare(b.name)
		);
	});

	let findInput: HTMLInputElement;
	// The matches open above the add box and push it down: keep it on screen.
	$effect(() => {
		if (matches.length > 0) void tick().then(() => findInput.scrollIntoView({ block: 'end' }));
	});

	async function add(exerciseId: string) {
		picking = false;
		query = '';
		await addExercise(kind, parentId, exerciseId);
	}

	/** Position of a set among the exercise's sets of the same type (warm-up or working), 0-based. */
	function nthOfType(ex: ExerciseView, i: number): number {
		const warm = ex.sets[i].warmup;
		return ex.sets.slice(0, i).filter((s) => s.warmup === warm).length;
	}

	/** Save a routine set's reps and max reps, and show what was kept (a max alone becomes the reps). */
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

	const UP_ICON = 'M12 19V5M6 11l6-6 6 6';
	const DOWN_ICON = 'M12 5v14M6 13l6 6 6-6';

	/** The menu's move items: a lone exercise moves as a block; one in a superset moves within it, or moves the whole superset. */
	function moves(ex: ExerciseView, block: Block, bi: number, ei: number) {
		const first = bi === 0;
		const last = bi === blocks.length - 1;
		const moveAll = (dir: -1 | 1) => () => moveBlock(kind, parentId, ex.id, dir);
		if (!block.superset) {
			return [
				{ text: 'Move up', label: `Move ${ex.name} up`, icon: UP_ICON, disabled: first, run: moveAll(-1) },
				{ text: 'Move down', label: `Move ${ex.name} down`, icon: DOWN_ICON, disabled: last, run: moveAll(1) }
			];
		}
		const within = (dir: -1 | 1) => () => moveWithinSuperset(kind, parentId, ex.id, dir);
		return [
			{
				text: 'Up in superset',
				label: `Move ${ex.name} up in superset`,
				icon: UP_ICON,
				disabled: ei === 0,
				run: within(-1)
			},
			{
				text: 'Down in superset',
				label: `Move ${ex.name} down in superset`,
				icon: DOWN_ICON,
				disabled: ei === block.items.length - 1,
				run: within(1)
			},
			{
				text: 'Move superset up',
				label: 'Move superset up',
				icon: UP_ICON,
				disabled: first,
				run: moveAll(-1)
			},
			{
				text: 'Move superset down',
				label: 'Move superset down',
				icon: DOWN_ICON,
				disabled: last,
				run: moveAll(1)
			}
		];
	}

	/** Run a menu action and close the menu. */
	function pick(action: () => unknown) {
		menuFor = null;
		void action();
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

	/** Workout sets are checked off as they are performed; only checked sets count. */
	const checkable = $derived(kind === 'workout');
</script>

<svelte:window
	onclick={(e) => {
		const target = e.target as Element;
		if (typeFor && !target.closest('.set-cell')) typeFor = null;
		if (menuFor && !target.closest('.ex-menu')) menuFor = null;
		if (picking && !target.closest('.add-exercise')) picking = false;
	}}
	onkeydown={(e) => {
		if (e.key === 'Escape') {
			menuFor = typeFor = null;
			picking = false;
		}
	}}
/>

{#each blocks as block, bi (block.items[0].id)}
	<div
		class:superset={block.superset}
		class="block"
		style:--ss={supersetColor[bi]}
		in:slide={{ duration: rowMs() }}
		out:slideOut={{ duration: rowMs() }}
		animate:flip={{ duration: rowMs() }}
	>
		{#if block.superset}
			<span class="badge">Superset</span>
		{/if}

		{#each block.items as ex, ei (ex.id)}
			{@const finished = allDone(ex)}
			{@const collapsed = finished && !expanded[ex.id]}
			{@const progression = progressions.current[ex.id]}
			<section
				aria-label={ex.name}
				class:collapsed
				in:slide={{ duration: rowMs() }}
				out:slideOut={{ duration: rowMs() }}
				animate:flip={{ duration: rowMs() }}
			>
				<header>
					{#if ex.equipment}
						<span class="equipment"><EquipmentIcon equipment={ex.equipment} /></span>
					{/if}
					{#if finished}
						<!-- Every set is checked off: the name folds the sets away and opens them again. -->
						<h2>
							<button
								class="fold"
								aria-expanded={!collapsed}
								onclick={() => (expanded[ex.id] = !expanded[ex.id])}
							>
								<span class="fold-name">{ex.name}</span>
								{#if collapsed}
									<span class="fold-summary" transition:fade={{ duration: foldMs }}>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
										{ex.sets.length}
										{plural(ex.sets.length, 'set')}
									</span>
								{/if}
								<svg class="chevron" viewBox="0 0 24 24" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
							</button>
						</h2>
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
							<!-- Last time every set reached the top of its rep range: the menu offers more weight. -->
							{#if progression?.suggested}
								<span class="menu-badge" data-testid="progression-badge"></span>
							{/if}
						</button>
						{#if menuFor === ex.id}
							<div
								class="dropdown"
								class:up={menuUp}
								role="menu"
								aria-label="{ex.name} options"
								{@attach placeMenu}
							>
								{#if progression}
									<button
										role="menuitem"
										class:suggested={progression.suggested}
										onclick={() => pick(() => applyProgression(progression))}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 17l6-6 4 4 8-8M15 7h6v6" /></svg>
										Add {kg(progression.step_kg)} kg
									</button>
								{/if}
								<!-- Extra sets are rare in a workout, so there it lives here instead of under the sets. -->
								{#if kind === 'workout'}
									<button role="menuitem" onclick={() => pick(() => addSet(kind, ex.id))}>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
										Add set
									</button>
								{/if}
								{#each moves(ex, block, bi, ei) as m (m.text)}
									<button
										role="menuitem"
										aria-label={m.label}
										disabled={m.disabled}
										onclick={() => pick(m.run)}
									>
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d={m.icon} /></svg>
										{m.text}
									</button>
								{/each}
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

				{#if !collapsed}
					<!-- A routine plans sets without weights: a workout takes them from the last session. -->
					<div class="fold-body" transition:slide={{ duration: foldMs }}>
						<div class="sets" class:plan={!checkable}>
							<span class="th">Set</span>
							{#if checkable}
								<span class="th">
									<svg class="kg-icon" viewBox="0 0 24 24" aria-hidden="true">
										<path d="M2 12h20M6 8v8M18 8v8M9 9.5v5M15 9.5v5" />
									</svg>
									kg
								</span>
							{/if}
							<span class="th">{ex.timed ? 'Time' : 'Reps'}</span>
							<span class="th"></span>

							{#each ex.sets as set, si (set.id)}
								{@const done = checkable && set.done}
								<div class="set-row" in:slide={{ duration: rowMs() }} out:slideOut={{ duration: rowMs() }}>
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
									{#if checkable}
										<input
											class:done
											inputmode="decimal"
											aria-label="Weight set {si + 1}"
											placeholder="0"
											value={set.weight_kg ? kg(set.weight_kg) : ''}
											onchange={(e) =>
												updateSet(kind, set.id, { weight_kg: parseWeight(e.currentTarget.value) })}
										/>
									{/if}
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
											<!-- Ticking a set off pops the tick in. -->
											{#key set.done}
												<svg
													viewBox="0 0 24 24"
													aria-hidden="true"
													in:scale={{ start: 0.4, duration: set.done ? motion(320) : 0, easing: backOut }}
													><path d="M5 12.5l4.5 4.5L19 7.5" /></svg
												>
											{/key}
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
												{durationLabel(set.rest_seconds)}
											</span>
											<select
												aria-label="Rest after set {si + 1}"
												value={set.rest_seconds}
												onchange={(e) =>
													updateSet(kind, set.id, { rest_seconds: Number(e.currentTarget.value) })}
											>
												{#each restOptions(set.rest_seconds) as sec (sec)}
													<option value={sec}>{sec === 0 ? 'No rest' : durationLabel(sec)}</option>
												{/each}
											</select>
										</label>
									{/if}
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if kind === 'routine'}
					<button class="add-set" onclick={() => addSet(kind, ex.id)}>
						<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
						Add set
					</button>
				{/if}
			</section>
		{/each}
	</div>
{/each}

<!-- The matches sit above the box, where the phone's keyboard does not cover them. -->
<div class="add-exercise">
	{#if matches.length > 0}
		<ul aria-label="Matching exercises">
			{#each matches as c (c.id)}
				<li>
					<button onclick={() => add(c.id)}>
						<EquipmentIcon equipment={c.equipment} />
						{c.name}
					</button>
				</li>
			{/each}
		</ul>
	{:else if picking}
		<p>No exercise with that name.</p>
	{/if}
	<svg class="plus" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
	<input
		aria-label="Find exercise"
		placeholder="Add exercise"
		autocomplete="off"
		bind:value={query}
		bind:this={findInput}
		onfocus={() => (picking = true)}
		onclick={() => (picking = true)}
		oninput={() => (picking = true)}
	/>
</div>

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

<style>
	section {
		padding: 1.25rem 0 1.5rem;
		border-bottom: 1px solid var(--border);
		transition: padding-bottom 0.25s;
	}

	/* The gap under the header lives inside the sliding part, so it folds away with the sets. */
	.fold-body {
		padding-top: 0.75rem;
	}

	.block.superset {
		border-left: 3px solid var(--ss);
		padding-left: 0.75rem;
		margin-top: 0.75rem;
	}

	.badge {
		display: block;
		padding-top: 0.25rem;
		color: var(--ss);
		font-weight: 700;
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.equipment {
		display: flex;
		flex: none;
		color: var(--muted);
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

	/* A finished exercise's name: tapping it shows or hides the sets. */
	.fold {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
		min-height: 0;
		padding: 0;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
	}

	.fold-name {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.fold-summary {
		display: flex;
		align-items: center;
		gap: 0.2rem;
		flex: none;
		color: var(--muted);
		font-size: 0.9rem;
		font-weight: 500;
	}

	.fold-summary svg {
		width: 16px;
		height: 16px;
		stroke: var(--accent);
		stroke-width: 3;
	}

	@media (prefers-reduced-motion: reduce) {
		section,
		.check-set,
		.sets input {
			transition: none;
		}
	}

	.fold .chevron {
		width: 20px;
		height: 20px;
		flex: none;
		margin-left: auto;
		color: var(--muted);
		transition: transform 0.2s;
	}

	.fold[aria-expanded='true'] .chevron {
		transform: rotate(180deg);
	}

	section.collapsed {
		padding-bottom: 0.5rem;
	}

	.ex-menu {
		position: relative;
		flex: none;
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
		position: relative;
		background: transparent;
		padding: 0;
		min-width: 40px;
		color: var(--text);
	}

	.menu-button[aria-expanded='true'] {
		visibility: hidden;
	}

	.menu-badge {
		position: absolute;
		top: 6px;
		right: 6px;
		width: 9px;
		height: 9px;
		border-radius: 50%;
		background: var(--accent);
	}

	.dropdown .suggested {
		color: var(--accent);
		font-weight: 600;
	}

	.menu-button svg {
		width: 22px;
		height: 22px;
		fill: currentColor;
		stroke: none;
	}

	/* Set table as a grid: Set | kg | Reps | check, or in a routine Set | Reps | delete */
	.sets {
		display: grid;
		grid-template-columns: 44px 1fr 1fr 44px;
		align-items: center;
		column-gap: 0.6rem;
	}

	/* One set across the columns. The gap above it is padding, so it folds away with the row. */
	.set-row {
		grid-column: 1 / -1;
		display: grid;
		grid-template-columns: subgrid;
		align-items: center;
		gap: 0.6rem;
		padding-top: 0.6rem;
	}

	.sets.plan {
		grid-template-columns: 44px 1fr 44px;
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

	.sets input {
		height: 44px;
		text-align: center;
		font-size: 1.05rem;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0 0.25rem;
		transition:
			background-color 0.2s,
			border-color 0.2s;
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
		transition:
			background-color 0.2s,
			color 0.2s,
			transform 0.1s;
	}

	.check-set:active {
		transform: scale(0.92);
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
		margin-top: 1.25rem;
	}

	.add-exercise input {
		height: 48px;
		background: var(--surface);
		border-color: transparent;
		border-radius: 12px;
		padding: 0 1rem;
	}

	.add-exercise ul {
		margin: 0 0 0.5rem;
		padding: 0.25rem;
		list-style: none;
		max-height: 40vh;
		overflow-y: auto;
		border-radius: 12px;
		background: var(--surface);
	}

	.add-exercise li button {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		width: 100%;
		padding: 0.6rem 0.75rem;
		background: transparent;
		color: var(--text);
		font-weight: 500;
		text-align: left;
	}

	.add-exercise p {
		margin: 0 0 0.5rem;
		color: var(--muted);
		font-size: 0.9rem;
	}
</style>
