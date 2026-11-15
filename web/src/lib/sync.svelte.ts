/**
 * Sync loop: push dirty rows, pull rows changed since our cursor, newest
 * updated_at wins. Rows the server refuses stay dirty (and are sent again, in
 * case a newer server accepts them) while the rest of the sync goes through. If Cloudflare Access answers instead of the API (redirect,
 * 401/403 or non-JSON), local data is kept and the UI asks for a reload.
 */
import { db, SYNCED_TABLES, type Synced, type SyncedTable } from './db';
import { onLocalChange } from './repo';

export type SyncStatus = 'idle' | 'syncing' | 'offline' | 'auth-expired' | 'error';

export const sync = $state({
	status: 'idle' as SyncStatus,
	/** Local changes the server refused on the last sync; they stay on this phone. */
	rejected: 0
});

type Wire = Omit<Synced, 'dirty'> & Record<string, unknown>;
type Changes = Partial<Record<SyncedTable, Wire[]>>;
/** A row the server refused, from the sync response. */
interface Rejection {
	table: string;
	id: string;
	error: string;
}

const CURSOR_KEY = 'cursor';
/** Meta key holding when the last sync succeeded, ms. */
export const LAST_SYNCED_KEY = 'last_synced';
const DEBOUNCE_MS = 800;

function table(name: SyncedTable) {
	return db.table<Synced & Record<string, unknown>, string>(name);
}

async function collectDirty(): Promise<Changes> {
	const changes: Changes = {};
	for (const name of SYNCED_TABLES) {
		const rows = await table(name).where('dirty').equals(1).toArray();
		if (rows.length > 0) changes[name] = rows.map(({ dirty: _dirty, ...rest }) => rest as Wire);
	}
	return changes;
}

/** True when the response came from Cloudflare Access rather than our API. */
function isAccessResponse(res: Response): boolean {
	if (res.type === 'opaqueredirect' || res.status === 401 || res.status === 403) return true;
	if (res.redirected) return true;
	return !(res.headers.get('content-type') ?? '').includes('application/json');
}

async function apply(sent: Changes, received: Changes, rejected: Rejection[], cursor: number) {
	const refused = new Set(rejected.map((r) => `${r.table}/${r.id}`));
	await db.transaction('rw', [...SYNCED_TABLES.map(table), db.meta], async () => {
		for (const name of SYNCED_TABLES) {
			const t = table(name);
			// Rows the server accepted (or already had newer) are clean unless edited meanwhile.
			for (const row of sent[name] ?? []) {
				if (refused.has(`${name}/${row.id}`)) continue;
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
		await db.meta.put({ key: LAST_SYNCED_KEY, value: Date.now() });
	});
}

async function syncOnce(): Promise<void> {
	if (!navigator.onLine) {
		sync.status = 'offline';
		return;
	}
	const changes = await collectDirty();
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
	const body = (await res.json()) as { cursor: number; changes: Changes; rejected?: Rejection[] };
	const rejected = body.rejected ?? [];
	if (rejected.length > 0) console.warn('sync: server refused rows', rejected);
	await apply(changes, body.changes ?? {}, rejected, body.cursor);
	sync.rejected = rejected.length;
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
