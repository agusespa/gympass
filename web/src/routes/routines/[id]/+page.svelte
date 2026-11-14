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
	<div class="details">
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
	<p class="muted">Routine not found. <a href="/">Back to routines</a></p>
{/if}

<style>
	.details {
		padding: 1rem 0 0.75rem;
		border-bottom: 1px solid var(--border);
	}

	.title {
		font-size: 1.6rem;
		font-weight: 700;
		background: transparent;
		border-color: transparent;
		padding: 0.25rem 0;
	}

	.actions {
		margin-top: 1.25rem;
	}
</style>
