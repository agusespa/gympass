<script lang="ts">
	import { goto } from '$app/navigation';
	import { live } from '#lib/live.svelte.ts';
	import {
		createRoutine,
		createWorkout,
		listRoutines,
		startWorkoutFromRoutine,
		type RoutineSummary
	} from '#lib/repo.ts';

	const routines = live(
		() => null,
		() => listRoutines(),
		[] as RoutineSummary[]
	);

	const todayLabel = new Date().toLocaleDateString(undefined, {
		weekday: 'long',
		day: 'numeric',
		month: 'short'
	});

	/** Stable hue per routine name, so each routine keeps its colour. */
	function hue(name: string): number {
		let h = 0;
		for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) % 360;
		return h;
	}

	function initials(name: string): string {
		const words = name
			.trim()
			.split(/[\s-]+/)
			.filter(Boolean);
		return ((words[0]?.[0] ?? '') + (words[1]?.[0] ?? '')).toUpperCase() || '?';
	}

	async function startEmpty() {
		const id = await createWorkout();
		await goto(`/workouts/${id}`);
	}

	async function newRoutine() {
		const id = await createRoutine();
		await goto(`/routines/${id}`);
	}

	async function start(id: string) {
		const workoutId = await startWorkoutFromRoutine(id);
		await goto(`/workouts/${workoutId}`);
	}
</script>

<p class="eyebrow">{todayLabel}</p>
<h1 class="large-title">Workout</h1>

<div class="section">
	<h2>Routines</h2>
	{#if routines.current.length > 0}<span class="count">{routines.current.length}</span>{/if}
	<span class="spacer"></span>
	<button class="new" aria-label="New routine" onclick={newRoutine}>
		<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v12M6 12h12" /></svg>
		New
	</button>
</div>

{#if routines.loaded && routines.current.length === 0}
	<button class="empty" onclick={newRoutine}>
		<strong>No routines yet</strong>
		<span>Create one here, or save a finished workout as a routine.</span>
	</button>
{:else}
	<ul class="routines">
		{#each routines.current as r (r.id)}
			<li style="--h: {hue(r.name)}">
				<a href="/routines/{r.id}">
					<span class="avatar" aria-hidden="true">{initials(r.name)}</span>
					<span class="info">
						<span class="name">{r.name}</span>
						<span class="meta">
							{#if r.exercises.length > 0}
								<b>{r.exercises.length}</b>
								{r.exercises.length === 1 ? 'exercise' : 'exercises'} · {r.exercises.join(', ')}
							{:else}
								No exercises yet
							{/if}
						</span>
					</span>
				</a>
				<button class="play" aria-label="Start {r.name}" onclick={() => start(r.id)}>
					<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5.5v13l10.5-6.5z" /></svg>
				</button>
			</li>
		{/each}
	</ul>
{/if}

<button class="empty-workout" onclick={startEmpty}>
	<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v12M6 12h12" /></svg>
	Start empty workout
</button>

<style>
	svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.eyebrow {
		margin: 0.25rem 0 0;
		font-size: 0.78rem;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
	}

	.section {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		margin: 0.25rem 0 0.6rem;
	}

	.section h2 {
		font-size: 1.05rem;
		font-weight: 700;
	}

	.count {
		min-width: 20px;
		padding: 0 6px;
		border-radius: 999px;
		background: var(--surface);
		color: var(--muted);
		font-size: 0.75rem;
		font-weight: 600;
		line-height: 20px;
		text-align: center;
	}

	.new {
		display: flex;
		align-items: center;
		gap: 0.2rem;
		min-height: 30px;
		padding: 0 0.7rem 0 0.5rem;
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		font-size: 0.85rem;
		font-weight: 600;
	}

	.new svg {
		width: 16px;
		height: 16px;
	}

	.empty {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		width: 100%;
		padding: 1.1rem;
		border: 1.5px dashed var(--border);
		border-radius: 16px;
		background: transparent;
		text-align: left;
		font-size: 0.9rem;
	}

	.empty span {
		color: var(--muted);
	}

	.routines {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.routines li {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 0.6rem 0.6rem 0.7rem;
		border-radius: 16px;
		background: var(--surface);
	}

	.routines a {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 0.75rem;
		color: inherit;
	}

	.avatar {
		flex-shrink: 0;
		display: grid;
		place-items: center;
		width: 40px;
		height: 40px;
		border-radius: 12px;
		background: hsl(var(--h) 70% 55% / 0.18);
		color: hsl(var(--h) 75% 60%);
		font-size: 0.85rem;
		font-weight: 800;
		letter-spacing: 0.02em;
	}

	.info {
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	.name {
		font-size: 0.98rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		font-size: 0.8rem;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta b {
		color: var(--text);
		font-weight: 600;
	}

	.play {
		flex-shrink: 0;
		display: grid;
		place-items: center;
		width: 38px;
		height: 38px;
		min-height: 0;
		padding: 0;
		border-radius: 50%;
		background: var(--accent);
		color: #fff;
		transition: transform 0.12s;
	}

	.play:active {
		transform: scale(0.92);
	}

	.play svg {
		width: 18px;
		height: 18px;
		fill: currentColor;
		stroke-width: 1.5;
		margin-left: 2px;
	}

	.empty-workout {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.3rem;
		margin: 1rem auto 0;
		min-height: 36px;
		padding: 0 0.9rem;
		background: transparent;
		color: var(--muted);
		font-size: 0.85rem;
		font-weight: 500;
	}

	.empty-workout svg {
		width: 15px;
		height: 15px;
	}
</style>
