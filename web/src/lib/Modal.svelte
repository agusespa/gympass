<script lang="ts">
	import type { Snippet } from 'svelte';

	/** Centered alert-style modal; `buttons` gets `close` so a button can dismiss it. */
	let { title, message, buttons }: { title: string; message: string; buttons: Snippet<[() => void]> } =
		$props();

	let dialog: HTMLDialogElement;

	export function open() {
		dialog.showModal();
	}

	export function close() {
		dialog.close();
	}
</script>

<!-- Tapping the backdrop (the dialog element itself, outside the panel) cancels. -->
<dialog
	bind:this={dialog}
	aria-label={title}
	onclick={(e) => {
		if (e.target === dialog) dialog.close();
	}}
>
	<div class="panel">
		<p class="title">{title}</p>
		<p class="message">{message}</p>
		<div class="buttons">{@render buttons(close)}</div>
	</div>
</dialog>

<style>
	dialog {
		width: min(320px, calc(100vw - 2rem));
		padding: 0;
		border: none;
		border-radius: var(--radius);
		background: var(--surface);
		color: var(--text);
	}

	dialog::backdrop {
		background: rgba(0, 0, 0, 0.5);
	}

	.panel {
		padding: 1.25rem 1rem 1rem;
		text-align: center;
	}

	.title {
		margin: 0;
		font-size: 1.1rem;
		font-weight: 600;
	}

	.message {
		margin: 0.4rem 0 1.1rem;
		color: var(--muted);
	}

	.buttons {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
	}

	.buttons :global(.destructive) {
		background: var(--danger);
		color: #fff;
		font-weight: 600;
	}
</style>
