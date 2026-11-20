<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import NameInput from '#lib/NameInput.svelte';
	import RoutineIcon from '#lib/RoutineIcon.svelte';
	import { ROUTINE_ICONS } from '#lib/routine-icons.ts';
	import TopBar from '#lib/TopBar.svelte';
	import { live } from '#lib/live.svelte.ts';
	import { deleteRoutine, getRoutine, updateRoutine } from '#lib/repo.ts';
	import type { Routine } from '#lib/db.ts';

	const id = $derived(page.params.id ?? '');
	const routine = live(
		() => id,
		(id) => getRoutine(id),
		undefined as Routine | undefined
	);

	async function remove() {
		await deleteRoutine(id);
		await goto('/');
	}
</script>

{#if routine.current}
	<TopBar title="Edit routine" fallback="/" />
	<div class="page-details">
		<NameInput
			label="Routine name"
			value={routine.current.name}
			onsave={(name) => updateRoutine(id, { name })}
		/>
	</div>

	<div class="icons" role="group" aria-label="Icon">
		{#each [{ id: '', label: 'Default' } as const, ...ROUTINE_ICONS] as opt (opt.id)}
			<button
				class="icon-option"
				aria-label="{opt.label} icon"
				aria-pressed={routine.current.icon === opt.id}
				onclick={() => updateRoutine(id, { icon: opt.id })}
			>
				<span class="tile"><RoutineIcon icon={opt.id} /></span>
				{opt.label}
			</button>
		{/each}
	</div>

	{#key id}
		<ExerciseEditor kind="routine" parentId={id} />
	{/key}

	<div class="row page-actions">
		<ConfirmButton label="Delete routine" onconfirm={remove} />
		<span class="spacer"></span>
		<!-- Edits already autosave; this is just a clear way out. -->
		<button class="primary" onclick={() => goto('/')}>Done</button>
	</div>
{:else if routine.loaded}
	<p class="muted">Routine not found. <a href="/">Back to routines</a></p>
{/if}

<style>
	/* One scrolling row, so the exercises stay close to the top. */
	.icons {
		display: flex;
		gap: 0.4rem;
		margin: 0 -1rem 1rem;
		padding: 0 1rem 0.25rem;
		overflow-x: auto;
		scrollbar-width: none;
	}

	.icon-option {
		flex: none;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.3rem;
		width: 64px;
		min-height: 0;
		padding: 0;
		background: none;
		color: var(--muted);
		font-size: 0.7rem;
		font-weight: 500;
		line-height: 1.15;
		text-align: center;
	}

	.tile {
		width: 52px;
		height: 52px;
		padding: 6px;
		border-radius: 14px;
		background: var(--surface);
		color: var(--text);
	}

	.icon-option[aria-pressed='true'] {
		color: var(--accent);
		font-weight: 600;
	}

	.icon-option[aria-pressed='true'] .tile {
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		box-shadow: inset 0 0 0 2px var(--accent);
		color: var(--accent);
	}
</style>
