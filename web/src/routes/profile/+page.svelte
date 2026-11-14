<script lang="ts">
	import { longDate } from '#lib/format.ts';
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
		<strong>{w.name}</strong>
		<span class="muted date">{longDate(w.date)}</span>
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
