<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ConfirmButton from '#lib/ConfirmButton.svelte';
	import ExerciseEditor from '#lib/ExerciseEditor.svelte';
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
		await goto('/routines');
	}
</script>

{#if routine.current}
	<div class="card">
		<input
			class="title"
			aria-label="Routine name"
			value={routine.current.name}
			onchange={(e) => {
				const v = e.currentTarget.value.trim();
				if (v) updateRoutine(id, { name: v });
			}}
		/>
	</div>

	{#key id}
		<ExerciseEditor kind="routine" parentId={id} />
	{/key}

	<div class="row actions">
		<span class="spacer"></span>
		<ConfirmButton label="Delete routine" onconfirm={remove} />
	</div>
{:else if routine.loaded}
	<p class="muted">Routine not found. <a href="/routines">Back to routines</a></p>
{/if}

<style>
	.title {
		font-size: 1.3rem;
		font-weight: 700;
	}

	.actions {
		margin-top: 1rem;
	}
</style>
