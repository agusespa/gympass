<script lang="ts">
	import { version } from '$app/env';
	import TopBar from '#lib/TopBar.svelte';
	import { catalogExercise } from '#lib/catalog.ts';
	import { today } from '#lib/dates.ts';
	import { db, SYNCED_TABLES } from '#lib/db.ts';
	import { MAX_WEEKLY_GOAL } from '#lib/limits.ts';
	import { live } from '#lib/live.svelte.ts';
	import { getWeeklyGoal, setWeeklyGoal } from '#lib/repo.ts';
	import { LAST_SYNCED_KEY, sync, syncNeedsAttention, syncNow, type SyncStatus } from '#lib/sync.svelte.ts';

	const goal = live(getWeeklyGoal, 0);
	const lastSynced = live(async () => Number((await db.meta.get(LAST_SYNCED_KEY))?.value ?? 0), 0);

	// Re-renders "x min ago" while the screen is open.
	let now = $state(Date.now());
	$effect(() => {
		const timer = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(timer);
	});

	const statusText: Record<SyncStatus, string> = {
		idle: 'Up to date',
		syncing: 'Syncing…',
		offline: 'Offline',
		'auth-expired': 'Login expired',
		error: 'Sync failed'
	};

	// Refused changes leave the status idle, which would otherwise read "Up to date".
	const status = $derived(
		sync.rejected > 0 && sync.status === 'idle' ? 'Changes refused' : statusText[sync.status]
	);
	const attention = $derived(syncNeedsAttention());
	const changes = (n: number) => (n === 1 ? '1 change is' : `${n} changes are`);

	function ago(ms: number): string {
		if (!ms) return 'Never';
		const min = Math.round((now - ms) / 60_000);
		if (min < 1) return 'Just now';
		if (min < 60) return `${min} min ago`;
		const d = new Date(ms);
		const sameDay = d.toDateString() === new Date(now).toDateString();
		return sameDay
			? d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
			: d.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });
	}

	// SvelteKit's build version is the build's timestamp; it tells two builds of one release apart.
	const builtMs = Number(version);
	const builtAt = Number.isFinite(builtMs)
		? new Date(builtMs).toLocaleString(undefined, {
				day: 'numeric',
				month: 'short',
				year: 'numeric',
				hour: 'numeric',
				minute: '2-digit'
			})
		: null;

	function download(name: string, type: string, body: string) {
		const url = URL.createObjectURL(new Blob([body], { type }));
		const a = document.createElement('a');
		a.href = url;
		a.download = name;
		a.click();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	}

	/** Everything on this phone, without deleted rows or local sync flags. */
	async function exportJson() {
		const data: Record<string, unknown[]> = {};
		for (const name of SYNCED_TABLES) {
			const rows = await db.table(name).toArray();
			data[name] = rows
				.filter((r) => !r.deleted)
				.map(({ dirty: _dirty, deleted: _deleted, ...rest }) => rest);
		}
		download(`gym-${today()}.json`, 'application/json', JSON.stringify(data, null, 2));
	}

	const csvCell = (v: unknown) => {
		const s = String(v ?? '');
		return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
	};

	/** One row per set of every finished workout, oldest first. */
	async function exportCsv() {
		const workouts = new Map(
			(await db.workouts.toArray()).filter((w) => !w.deleted && w.finished_at != null).map((w) => [w.id, w])
		);
		const exercises = new Map(
			(await db.workout_exercises.toArray())
				.filter((e) => !e.deleted && workouts.has(e.workout_id))
				.map((e) => [e.id, e])
		);
		const rows = (await db.sets.toArray())
			.filter((s) => !s.deleted && exercises.has(s.workout_exercise_id))
			.map((s) => {
				const e = exercises.get(s.workout_exercise_id)!;
				const w = workouts.get(e.workout_id)!;
				return { w, e, s };
			})
			.sort(
				(a, b) =>
					a.w.date.localeCompare(b.w.date) ||
					a.w.started_at - b.w.started_at ||
					a.e.position - b.e.position ||
					a.s.position - b.s.position
			);
		const header = [
			'date',
			'workout',
			'exercise',
			'set',
			'weight_kg',
			'reps',
			'warmup',
			'done',
			'duration_seconds'
		];
		const lines = [header.join(',')];
		for (const { w, e, s } of rows) {
			lines.push(
				[
					w.date,
					w.name,
					catalogExercise(e.exercise_id)?.name ?? '',
					s.position,
					s.weight_kg,
					s.reps,
					s.warmup,
					s.done,
					s.duration_seconds
				]
					.map(csvCell)
					.join(',')
			);
		}
		download(`gym-sets-${today()}.csv`, 'text/csv', lines.join('\n') + '\n');
	}
</script>

<TopBar fallback="/profile" title="Settings" />

<h2 class="group">Weekly goal</h2>
<div class="card rows">
	<div class="row">
		<span>Workouts per week</span>
		<span class="stepper">
			<button
				aria-label="Fewer workouts per week"
				disabled={goal.current <= 0}
				onclick={() => setWeeklyGoal(goal.current - 1)}>−</button
			>
			<span class="goal" data-testid="weekly-goal">{goal.current || 'Off'}</span>
			<button
				aria-label="More workouts per week"
				disabled={goal.current >= MAX_WEEKLY_GOAL}
				onclick={() => setWeeklyGoal(goal.current + 1)}>+</button
			>
		</span>
	</div>
</div>
<p class="muted note">
	Weeks that fall short show in red on your profile, and the streak counts weeks you hit it.
</p>

<h2 class="group">
	Sync{#if attention}<span class="sync-dot" aria-hidden="true"></span>{/if}
</h2>
<div class="card rows">
	<div class="row">
		<span>Status</span>
		<span class={attention ? 'danger' : 'muted'} data-testid="sync-status">{status}</span>
	</div>
	<div class="row">
		<span>Last synced</span>
		<span class="muted" data-testid="last-synced">{ago(lastSynced.current)}</span>
	</div>
	<div class="row">
		<span>Changes not synced</span>
		<span class="muted" data-testid="pending">{sync.pending}</span>
	</div>
	{#if sync.rejected > 0}
		<div class="row">
			<span>Refused by the server</span>
			<span class="danger" data-testid="rejected">{sync.rejected}</span>
		</div>
	{/if}
	{#if sync.status === 'auth-expired'}
		<button class="row action" onclick={() => location.reload()}>Log in again</button>
	{:else}
		<button class="row action" disabled={sync.status === 'syncing'} onclick={() => syncNow()}>Sync now</button
		>
	{/if}
</div>
<p class="muted note" class:alert={attention} data-testid="sync-note">
	{#if sync.status === 'auth-expired'}
		Your login has expired. Your changes are kept on this phone and sync once you log in again.
	{:else if attention}
		<!-- Refused changes and an offline or failed sync can happen together; say both. -->
		{#if sync.rejected > 0}
			The server refused {sync.rejected === 1 ? 'a change' : 'some changes'}. {sync.rejected === 1
				? "It's"
				: "They're"} kept on this phone and sent again on every sync.
		{/if}
		{#if sync.pending > 0 && sync.status === 'offline'}
			You're offline. {changes(sync.pending)} waiting on this phone and will sync once you're back online.
		{:else if sync.pending > 0 && sync.status === 'error'}
			The last sync failed. {changes(sync.pending)} waiting on this phone; tap Sync now to try again.
		{/if}
	{:else}
		Everything is kept on this phone too, so you can train offline and sync later.
	{/if}
</p>

<h2 class="group">Export</h2>
<div class="card rows">
	<button class="row action" onclick={exportCsv}>Sets as CSV</button>
	<button class="row action" onclick={exportJson}>All data as JSON</button>
</div>
<p class="muted note">
	CSV has one row per set of every finished workout, for spreadsheets. JSON has workouts and routines in full.
</p>

<p class="muted version" data-testid="app-version">
	Version {__APP_VERSION__}{#if builtAt}<br />Built {builtAt}{/if}
</p>

<style>
	.group {
		font-size: 0.8rem;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		color: var(--muted);
		margin: 1.5rem 0.9rem 0.4rem;
	}

	.rows {
		padding: 0 0 0 0.9rem;
		margin-bottom: 0.4rem;
	}

	.row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		min-height: 44px;
		padding: 0 0.9rem 0 0;
		font-variant-numeric: tabular-nums;
	}

	.row + .row {
		border-top: 1px solid var(--border);
	}

	.action {
		width: 100%;
		border-radius: 0;
		background: none;
		color: var(--accent);
		text-align: left;
	}

	.danger {
		color: var(--danger);
	}

	/* Same dot as the Profile tab and settings gear, so it's clear which section they point to. */
	.sync-dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		margin-left: 0.4rem;
		border-radius: 50%;
		background: var(--danger);
		vertical-align: 0.1em;
	}

	.note.alert {
		color: var(--text);
	}

	.stepper {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.stepper button {
		width: 36px;
		min-height: 36px;
		padding: 0;
		border-radius: 999px;
		background: var(--bar-active);
		color: var(--accent);
		font-size: 1.2rem;
		line-height: 1;
	}

	.stepper button:disabled {
		color: var(--muted);
	}

	.goal {
		min-width: 2.5rem;
		text-align: center;
		font-weight: 600;
	}

	.note {
		font-size: 0.8rem;
		margin: 0 0.9rem;
	}

	.version {
		font-size: 0.8rem;
		text-align: center;
		margin: 2.5rem 0 0;
		font-variant-numeric: tabular-nums;
	}
</style>
