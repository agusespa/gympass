<script lang="ts">
	import { shortDate } from './format';
	import type { Point } from './stats';

	/**
	 * One line over time, points placed by date. Tapping or hovering a point
	 * selects it; `format` writes a value for the header and labels.
	 */
	let { points, label, format }: { points: Point[]; label: string; format: (v: number) => string } = $props();

	const H = 140;
	const PAD = { top: 12, right: 10, bottom: 8, left: 10 };
	let width = $state(0);
	let picked = $state<number | null>(null);
	const index = $derived(Math.min(picked ?? points.length - 1, points.length - 1));

	const time = (d: string) => new Date(d + 'T00:00:00').getTime();
	const geometry = $derived.by(() => {
		const values = points.map((p) => p.value);
		let lo = Math.min(...values);
		let hi = Math.max(...values);
		// Pad the range so a flat line sits mid-plot and changes don't hug the edges.
		const span = hi - lo || Math.max(hi * 0.1, 1);
		lo = Math.max(0, lo - span * 0.15);
		hi = hi + span * 0.15;
		const t0 = time(points[0].date);
		const t1 = time(points[points.length - 1].date);
		const plotW = Math.max(0, width - PAD.left - PAD.right);
		const plotH = H - PAD.top - PAD.bottom;
		const xy = points.map((p) => ({
			x: PAD.left + (t1 === t0 ? plotW / 2 : ((time(p.date) - t0) / (t1 - t0)) * plotW),
			y: PAD.top + (1 - (p.value - lo) / (hi - lo)) * plotH
		}));
		return { xy, path: xy.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join('') };
	});
	const shown = $derived(points[index]);
</script>

<section class="card chart" aria-label={label}>
	<div class="head">
		<span class="muted">{label}</span>
		<strong data-testid="progress-value">{format(shown.value)}</strong>
		<span class="muted">{shortDate(shown.date)}</span>
	</div>
	<div class="plot" bind:clientWidth={width}>
		{#if width > 0}
			<svg
				{width}
				height={H}
				role="img"
				aria-label="{label}: {points.map((p) => `${shortDate(p.date)} ${format(p.value)}`).join(', ')}"
			>
				<line class="base" x1={PAD.left} x2={width - PAD.right} y1={H - PAD.bottom} y2={H - PAD.bottom} />
				{#if points.length > 1}
					<path class="line" d={geometry.path} />
				{/if}
				{#each geometry.xy as p, i (i)}
					<circle class="dot" class:selected={i === index} cx={p.x} cy={p.y} r={i === index ? 5 : 3.5} />
				{/each}
				{#if geometry.xy[index]}
					<line
						class="cross"
						x1={geometry.xy[index].x}
						x2={geometry.xy[index].x}
						y1={PAD.top}
						y2={H - PAD.bottom}
					/>
				{/if}
			</svg>
			<!-- Hit targets wider than the dots: each covers the space halfway to its neighbours. -->
			<div class="hits" aria-hidden="true">
				{#each geometry.xy as p, i (i)}
					{@const prev = i ? (geometry.xy[i - 1].x + p.x) / 2 : 0}
					{@const next = i < geometry.xy.length - 1 ? (geometry.xy[i + 1].x + p.x) / 2 : width}
					<button
						tabindex="-1"
						aria-label={shortDate(points[i].date)}
						style:left="{prev}px"
						style:width="{next - prev}px"
						onclick={() => (picked = i)}
						onpointerenter={(e) => e.pointerType === 'mouse' && (picked = i)}
					></button>
				{/each}
			</div>
		{/if}
	</div>
	{#if points.length > 1}
		<div class="axis muted" aria-hidden="true">
			<span>{shortDate(points[0].date)}</span>
			<span>{shortDate(points[points.length - 1].date)}</span>
		</div>
	{/if}
</section>

<style>
	.chart {
		padding: 0.9rem 0.9rem 0.6rem;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
		font-variant-numeric: tabular-nums;
	}

	.head .muted {
		font-size: 0.85rem;
	}

	.head .muted:first-child {
		margin-right: auto;
	}

	.head strong {
		font-size: 1.25rem;
		font-weight: 700;
	}

	.plot {
		position: relative;
		height: 140px;
	}

	svg {
		display: block;
		overflow: visible;
	}

	.base {
		stroke: var(--separator);
		stroke-width: 1;
	}

	.cross {
		stroke: var(--separator);
		stroke-width: 1;
		stroke-dasharray: 3 3;
	}

	.line {
		fill: none;
		stroke: var(--accent);
		stroke-width: 2;
		stroke-linejoin: round;
		stroke-linecap: round;
	}

	/* Surface ring keeps dots legible where they sit on the line. */
	.dot {
		fill: var(--accent);
		stroke: var(--surface);
		stroke-width: 2;
	}

	.hits {
		position: absolute;
		inset: 0;
	}

	.hits button {
		position: absolute;
		top: 0;
		bottom: 0;
		min-height: 0;
		padding: 0;
		background: none;
		border-radius: 0;
	}

	.axis {
		display: flex;
		justify-content: space-between;
		padding-top: 0.35rem;
		font-size: 0.72rem;
	}
</style>
