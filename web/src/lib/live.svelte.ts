import { liveQuery } from 'dexie';
import { reportError } from './errors.ts';

/**
 * Reactive wrapper around a Dexie live query. `key` is read reactively; the
 * query re-runs when it changes or when the underlying tables change.
 */
export function live<K, T>(key: () => K, query: (k: K) => Promise<T>, initial: T) {
	let value = $state<T>(initial);
	let loaded = $state(false);
	$effect(() => {
		const k = key();
		const sub = liveQuery(() => query(k)).subscribe({
			next: (v) => {
				value = v;
				loaded = true;
			},
			error: reportError
		});
		return () => sub.unsubscribe();
	});
	return {
		get current() {
			return value;
		},
		get loaded() {
			return loaded;
		}
	};
}
