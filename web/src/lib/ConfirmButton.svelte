<script lang="ts">
	/**
	 * Destructive button that asks for a second tap instead of using window.confirm,
	 * which some browsers (and embedded webviews) block.
	 */
	let {
		label,
		confirmLabel = 'Confirm delete',
		onconfirm
	}: { label: string; confirmLabel?: string; onconfirm: () => unknown } = $props();

	let asking = $state(false);
</script>

{#if asking}
	<span class="confirm">
		<button onclick={() => (asking = false)}>Cancel</button>
		<button
			class="yes"
			onclick={async () => {
				asking = false;
				await onconfirm();
			}}>{confirmLabel}</button
		>
	</span>
{:else}
	<button class="danger" onclick={() => (asking = true)}>{label}</button>
{/if}

<style>
	.confirm {
		display: inline-flex;
		gap: 0.4rem;
	}

	.yes {
		background: var(--danger);
		color: #fff;
		font-weight: 600;
	}
</style>
