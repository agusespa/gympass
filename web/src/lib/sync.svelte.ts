/**
 * Sync loop: push dirty rows, pull rows changed since our cursor, newest
 * updated_at wins. If Cloudflare Access answers instead of the API (redirect,
 * 401/403 or non-JSON), local data is kept and the UI asks for a reload.
 */
import { db, SYNCED_TABLES, type Synced, type SyncedTable } from './db';
import { onLocalChange } from './repo';

export type SyncStatus = 'idle' | 'syncing' | 'offline' | 'auth-expired' | 'error';

export const sync = $state({ status: 'idle' as SyncStatus });

type Wire = Omit<Synced, 'dirty'> & Record<string, unknown>;
type Changes = Partial<Record<SyncedTable, Wire[]>>;

const CURSOR_KEY = 'cursor';
const DEBOUNCE_MS = 800;

function table(name: SyncedTable) {
	return db.table<Synced & Record<string, unknown>, string>(name);
}

async function collectDirty(): Promise<{ changes: Changes; count: number }> {
	const changes: Changes = {};
	let count = 0;
	for (const name of SYNCED_TABLES) {
		const rows = await table(name).where('dirty').equals(1).toArray();
		if (rows.length === 0) continue;
		changes[name] = rows.map(({ dirty: _dirty, ...rest }) => rest as Wire);
		count += rows.length;
	}
	return { changes, count };
}

/** True when the response came from Cloudflare Access rather than our API. */
function isAccessResponse(res: Response): boolean {
	if (res.type === 'opaqueredirect' || res.status === 401 || res.status === 403) return true;
	if (res.redirected) return true;
	return !(res.headers.get('content-type') ?? '').includes('application/json');
}

async function apply(sent: Changes, received: Changes, cursor: number) {
	await db.transaction('rw', [...SYNCED_TABLES.map(table), db.meta], async () => {
		for (const name of SYNCED_TABLES) {
			const t = table(name);
			// Rows the server accepted (or already had newer) are clean unless edited meanwhile.
			for (const row of sent[name] ?? []) {
				const local = await t.get(row.id);
				if (local && local.updated_at === row.updated_at) await t.update(row.id, { dirty: 0 });
			}
			for (const row of received[name] ?? []) {
				const local = await t.get(row.id);
				if (!local || row.updated_at > local.updated_at) {
					await t.put({ ...row, dirty: 0 } as Synced & Record<string, unknown>);
				}
			}
		}
		await db.meta.put({ key: CURSOR_KEY, value: cursor });
	});
}

async function syncOnce(): Promise<void> {
	if (!navigator.onLine) {
		sync.status = 'offline';
		return;
	}
	const { changes } = await collectDirty();
	const cursor = Number((await db.meta.get(CURSOR_KEY))?.value ?? 0);

	let res: Response;
	try {
		res = await fetch('/api/sync', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
			body: JSON.stringify({ cursor, changes }),
			credentials: 'same-origin',
			redirect: 'manual',
			cache: 'no-store'
		});
	} catch {
		sync.status = 'offline';
		return;
	}

	if (isAccessResponse(res)) {
		sync.status = 'auth-expired';
		return;
	}
	if (!res.ok) {
		sync.status = 'error';
		console.error('sync failed', res.status, await res.text().catch(() => ''));
		return;
	}
	const body = (await res.json()) as { cursor: number; changes: Changes };
	await apply(changes, body.changes ?? {}, body.cursor);
	sync.status = 'idle';
}

let running: Promise<void> | null = null;
let again = false;
let timer: ReturnType<typeof setTimeout> | undefined;

/** Run a sync now; if one is in flight, run another right after it. */
export function syncNow(): Promise<void> {
	if (running) {
		again = true;
		return running;
	}
	if (sync.status !== 'auth-expired') sync.status = 'syncing';
	running = (async () => {
		try {
			do {
				again = false;
				await syncOnce();
			} while (again && sync.status === 'idle');
		} catch (e) {
			console.error('sync error', e);
			sync.status = 'error';
		} finally {
			running = null;
		}
	})();
	return running;
}

function scheduleSync() {
	clearTimeout(timer);
	timer = setTimeout(() => void syncNow(), DEBOUNCE_MS);
}

let started = false;

/** Start syncing: now, after local edits, when back online and when the app is shown. */
export function startSync() {
	if (started) return;
	started = true;
	onLocalChange(scheduleSync);
	window.addEventListener('online', () => void syncNow());
	window.addEventListener('offline', () => (sync.status = 'offline'));
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') void syncNow();
	});
	void syncNow();
}
