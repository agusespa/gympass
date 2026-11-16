<script lang="ts">
	import type { Snippet } from 'svelte';
	import { motion } from './motion';

	/** Centered alert-style modal; `buttons` gets `close` so a button can dismiss it. */
	let { title, message, buttons }: { title: string; message: string; buttons: Snippet<[() => void]> } =
		$props();

	let dialog: HTMLDialogElement;
	/** Playing the closing animation; the dialog closes when it ends. */
	let closing = $state(false);

	export function open() {
		closing = false;
		dialog.showModal();
	}

	export function close() {
		if (!dialog.open || closing) return;
		const ms = motion(150);
		if (!ms) return dialog.close();
		closing = true;
		setTimeout(() => {
			if (!closing) return;
			closing = false;
			dialog.close();
		}, ms);
	}
</script>

<!-- Tapping the backdrop (the dialog element itself, outside the panel) cancels. -->
<dialog
	bind:this={dialog}
	aria-label={title}
	class:closing
	onclick={(e) => {
		if (e.target === dialog) close();
	}}
	oncancel={(e) => {
		// Escape: fade out like the other ways of closing.
		e.preventDefault();
		close();
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

	/* Fades in with the panel growing slightly, like the menus; shrinks back out on close. */
	dialog[open] {
		animation: dialog-in 0.2s cubic-bezier(0.2, 0.9, 0.3, 1.1);
	}

	dialog[open]::backdrop {
		animation: backdrop-in 0.2s ease-out;
	}

	dialog.closing {
		animation: dialog-out 0.15s ease-in forwards;
	}

	dialog.closing::backdrop {
		animation: backdrop-out 0.15s ease-in forwards;
	}

	@keyframes dialog-in {
		from {
			opacity: 0;
			transform: scale(0.92);
		}
	}

	@keyframes dialog-out {
		to {
			opacity: 0;
			transform: scale(0.95);
		}
	}

	@keyframes backdrop-in {
		from {
			opacity: 0;
		}
	}

	@keyframes backdrop-out {
		to {
			opacity: 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		dialog[open],
		dialog[open]::backdrop {
			animation: none;
		}
	}
</style>
