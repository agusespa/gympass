<script lang="ts">
	import { duration, longDate } from '#lib/format.ts';
	import { live } from '#lib/live.svelte.ts';
	import { listWorkouts, type WorkoutSummary } from '#lib/repo.ts';

	const workouts = live(
		() => null,
		() => listWorkouts(),
		[] as WorkoutSummary[]
	);
</script>

<h1 class="large-title">Profile</h1>

<h2 class="section-title">History</h2>

{#if workouts.loaded && workouts.current.length === 0}
	<p class="muted">No workouts yet.</p>
{/if}

{#each workouts.current as w (w.id)}
	<a class="card workout" href="/workouts/{w.id}">
		<strong>
			{w.name}
			{#if w.finished_at == null}<span class="badge">In progress</span>{/if}
		</strong>
		<span class="muted date">
			{longDate(w.date)}
			{#if w.started_at && w.finished_at && duration(w.finished_at - w.started_at)}
				· {duration(w.finished_at - w.started_at)}
			{/if}
		</span>
		{#if w.exercises.length > 0}
			<span class="muted summary">{w.exercises.join(', ')}</span>
		{/if}
	</a>
{/each}

<style>
	.section-title {
		font-size: 1.3rem;
		font-weight: 600;
		margin: 0.5rem 0 0.75rem;
	}

	.workout {
		display: block;
		padding: 1.1rem;
		color: inherit;
	}

	.workout:active {
		opacity: 0.7;
	}

	strong {
		display: block;
		font-size: 1.15rem;
		font-weight: 600;
	}

	.badge {
		margin-left: 0.35rem;
		padding: 0.1rem 0.45rem;
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		font-size: 0.72rem;
		font-weight: 600;
		vertical-align: middle;
	}

	.date {
		display: block;
		font-size: 0.9rem;
		margin-top: 0.15rem;
	}

	.summary {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		margin-top: 0.4rem;
	}
</style>
