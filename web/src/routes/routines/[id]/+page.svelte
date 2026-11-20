<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
	import NameInput from '#lib/NameInput.svelte';
	import RoutineIcon from '#lib/RoutineIcon.svelte';
	import { SPLITS, type Split } from '#lib/splits.ts';
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

	<!-- The split picks the routine's icon; a custom one is named here too. -->
	<section class="split">
		<span class="tile"><RoutineIcon split={routine.current.split} /></span>
		<label class="field">
			<span class="field-label">Split</span>
			<select
				aria-label="Split"
				value={routine.current.split}
				onchange={(e) => {
					const split = e.currentTarget.value as Split | '';
					updateRoutine(id, { split, split_name: split === 'custom' ? routine.current!.split_name : '' });
				}}
			>
				<option value="">None</option>
				{#each SPLITS as s (s.id)}
					<option value={s.id}>{s.label}</option>
				{/each}
			</select>
		</label>
		{#if routine.current.split === 'custom'}
			<input
				class="custom-name"
				aria-label="Split name"
				placeholder="Split name, e.g. Arms"
				value={routine.current.split_name}
				onchange={(e) => updateRoutine(id, { split_name: e.currentTarget.value.trim() })}
			/>
		{/if}
	</section>

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
	.split {
		display: grid;
		grid-template-columns: auto 1fr;
		align-items: center;
		gap: 0.6rem 0.75rem;
		margin: 0.9rem 0 1rem;
		padding: 0.6rem 0.75rem;
		border-radius: 14px;
		background: var(--surface);
	}

	.tile {
		width: 44px;
		height: 44px;
		padding: 5px;
		border-radius: 12px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		min-width: 0;
	}

	.field-label {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--muted);
	}

	/* Reads as a value with a chevron, not a form box. */
	select {
		appearance: none;
		width: 100%;
		min-height: 28px;
		padding: 0 1.4rem 0 0;
		border: none;
		background: transparent
			url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23888' stroke-width='2.4' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M7 10l5 5 5-5'/%3E%3C/svg%3E")
			right center / 18px no-repeat;
		font-size: 1.05rem;
		font-weight: 600;
	}

	.custom-name {
		grid-column: 1 / -1;
	}
</style>
