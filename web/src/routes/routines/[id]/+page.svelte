<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
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
		<input
			class="page-name"
			aria-label="Routine name"
			value={routine.current.name}
			onchange={(e) => {
				const v = e.currentTarget.value.trim();
				if (v) updateRoutine(id, { name: v });
			}}
		/>
	</div>

	<p class="hint muted">Fill in a max next to the reps to aim for a range, like 8 to 12.</p>

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
	.hint {
		margin: 0.25rem 0 0;
		font-size: 0.85rem;
	}
</style>
