<script lang="ts">
	/** A workout's or routine's name at the top of its page. Saved on change; an emptied field is not saved. */
	let { value, label, onsave }: { value: string; label: string; onsave: (name: string) => unknown } =
		$props();
</script>

<input
	class="page-name"
	aria-label={label}
	{value}
	onchange={(e) => {
		const name = e.currentTarget.value;
		if (name.trim()) onsave(name);
	}}
/>
