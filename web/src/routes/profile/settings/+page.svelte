<script lang="ts">
	import TopBar from '#lib/TopBar.svelte';
	import { db, SYNCED_TABLES } from '#lib/db.ts';
	import { live } from '#lib/live.svelte.ts';
	import { today } from '#lib/repo.ts';
	import { LAST_SYNCED_KEY, sync, syncNow, type SyncStatus } from '#lib/sync.svelte.ts';

	const lastSynced = live(
		() => null,
		async () => Number((await db.meta.get(LAST_SYNCED_KEY))?.value ?? 0),
		0
	);
	const pending = live(
		() => null,
		async () => {
			let n = 0;
			for (const name of SYNCED_TABLES) n += await db.table(name).where('dirty').equals(1).count();
			return n;
		},
		0
	);

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
					(a.w.started_at ?? 0) - (b.w.started_at ?? 0) ||
					a.e.position - b.e.position ||
					a.s.position - b.s.position
			);
		const header = ['date', 'workout', 'exercise', 'set', 'weight_kg', 'reps', 'warmup', 'done'];
		const lines = [header.join(',')];
		for (const { w, e, s } of rows) {
			lines.push(
				[w.date, w.name, e.name, s.position, s.weight_kg, s.reps, !!s.warmup, s.done ?? true]
					.map(csvCell)
					.join(',')
			);
		}
		download(`gym-sets-${today()}.csv`, 'text/csv', lines.join('\n') + '\n');
	}
</script>

<TopBar fallback="/profile" title="Settings" />

<h2 class="group">Sync</h2>
<div class="card rows">
	<div class="row">
		<span>Status</span>
		<span class="muted" data-testid="sync-status">{statusText[sync.status]}</span>
	</div>
	<div class="row">
		<span>Last synced</span>
		<span class="muted" data-testid="last-synced">{ago(lastSynced.current)}</span>
	</div>
	<div class="row">
		<span>Changes not synced</span>
		<span class="muted" data-testid="pending">{pending.current}</span>
	</div>
	<button class="row action" disabled={sync.status === 'syncing'} onclick={() => syncNow()}>Sync now</button>
</div>
<p class="muted note">Everything is kept on this phone too, so you can train offline and sync later.</p>

<h2 class="group">Export</h2>
<div class="card rows">
	<button class="row action" onclick={exportCsv}>Sets as CSV</button>
	<button class="row action" onclick={exportJson}>All data as JSON</button>
</div>
<p class="muted note">
	CSV has one row per set of every finished workout, for spreadsheets. JSON has workouts and routines in full.
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

	.note {
		font-size: 0.8rem;
		margin: 0 0.9rem;
	}
</style>
