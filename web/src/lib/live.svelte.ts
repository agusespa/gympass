import { liveQuery } from 'dexie';
import { reportError } from './errors.ts';

export interface Live<T> {
	readonly current: T;
	readonly loaded: boolean;
}

/**
 * Reactive wrapper around a Dexie live query. `key`, when given, is read
 * reactively; the query re-runs when it changes or when the underlying
 * tables change.
 */
export function live<T>(query: () => Promise<T>, initial: T): Live<T>;
export function live<K, T>(key: () => K, query: (k: K) => Promise<T>, initial: T): Live<T>;
export function live<K, T>(...args: [() => Promise<T>, T] | [() => K, (k: K) => Promise<T>, T]): Live<T> {
	const [key, query, initial]: [() => K | undefined, (k: K | undefined) => Promise<T>, T] =
		args.length === 2
			? [() => undefined, args[0], args[1]]
			: (args as [() => K, (k: K | undefined) => Promise<T>, T]);
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
