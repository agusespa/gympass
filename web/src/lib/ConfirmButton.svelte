<script lang="ts">
	import Modal from './Modal.svelte';

	/**
	 * Destructive button that asks in a modal instead of using window.confirm,
	 * which some browsers (and embedded webviews) block.
	 */
	let {
		label,
		confirmLabel = 'Confirm delete',
		message = "This can't be undone.",
		onconfirm
	}: { label: string; confirmLabel?: string; message?: string; onconfirm: () => unknown } = $props();

	let modal: Modal;
</script>

<button class="danger" onclick={() => modal.open()}>{label}</button>

<Modal bind:this={modal} title={`${label}?`} {message}>
	{#snippet buttons(close)}
		<button onclick={close}>Cancel</button>
		<button
			class="destructive"
			onclick={async () => {
				close();
				await onconfirm();
			}}>{confirmLabel}</button
		>
	{/snippet}
</Modal>
