<script lang="ts">
	/** A workout's or routine's name at the top of its page. Saved on change; an emptied field is not saved. */
	let {
		value,
		label,
		onsave,
		compact = false
	}: {
		value: string;
		label: string;
		onsave: (name: string) => unknown;
		/** Smaller text, for a name that sits in the top bar. */
		compact?: boolean;
	} = $props();
</script>

<input
	class="page-name"
	class:compact
	aria-label={label}
	{value}
	onchange={(e) => {
		const name = e.currentTarget.value;
		if (name.trim()) onsave(name);
	}}
/>
