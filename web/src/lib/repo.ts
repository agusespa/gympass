/**
 * All reads and writes of local data. Every write stamps updated_at, marks the
 * row dirty and notifies listeners (the sync loop).
 */
import type { Table } from 'dexie';
import {
	db,
	DEFAULT_REST_SECONDS,
	type Exercise,
	type Routine,
	type SetRow,
	type Synced,
	type Workout
} from './db';

export type Kind = 'workout' | 'routine';

export interface SetView {
	id: string;
	position: number;
	reps: number;
	weight_kg: number;
	warmup: boolean;
	rest_seconds: number;
}

function toSetView(s: SetRow & { id: string }): SetView {
	return {
		id: s.id,
		position: s.position,
		reps: s.reps,
		weight_kg: s.weight_kg,
		warmup: !!s.warmup,
		rest_seconds: s.rest_seconds ?? DEFAULT_REST_SECONDS
	};
}

export interface ExerciseView {
	id: string;
	name: string;
	position: number;
	superset_id: string | null;
	sets: SetView[];
}

/** A single exercise, or two or more adjacent exercises sharing a superset_id. */
export interface Block {
	superset: boolean;
	items: ExerciseView[];
}

// ---------- plumbing ----------

const listeners = new Set<() => void>();

/** Register a callback for local writes. Returns an unsubscribe function. */
export function onLocalChange(fn: () => void): () => void {
	listeners.add(fn);
	return () => listeners.delete(fn);
}

function changed() {
	for (const fn of listeners) fn();
}

let lastStamp = 0;
/** Strictly increasing ms timestamp, so two quick edits of one row never tie. */
export function stamp(): number {
	lastStamp = Math.max(Date.now(), lastStamp + 1);
	return lastStamp;
}

export function uuid(): string {
	return crypto.randomUUID();
}

export function today(): string {
	const d = new Date();
	const m = String(d.getMonth() + 1).padStart(2, '0');
	const day = String(d.getDate()).padStart(2, '0');
	return `${d.getFullYear()}-${m}-${day}`;
}

function fresh<T extends object>(row: T): T & Synced {
	return { ...row, id: uuid(), updated_at: stamp(), deleted: false, dirty: 1 } as T & Synced;
}

const allTables = () => [
	db.workouts,
	db.workout_exercises,
	db.sets,
	db.routines,
	db.routine_exercises,
	db.routine_sets
];

async function write<T>(fn: () => Promise<T>): Promise<T> {
	const result = await db.transaction('rw', allTables(), fn);
	changed();
	return result;
}

/** Apply a patch to one row, stamping it. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function patch(table: Table<any, string>, id: string, changes: Record<string, unknown>) {
	await table.update(id, { ...changes, updated_at: stamp(), dirty: 1 });
}

function live<T extends Synced>(rows: T[]): T[] {
	return rows.filter((r) => !r.deleted);
}

const byPosition = (a: { position: number }, b: { position: number }) => a.position - b.position;

// Tables differ per kind only in names; this keeps the rest of the code shared.
interface KindTables {
	ex: Table<Exercise & Record<string, unknown>, string>;
	sets: Table<SetRow & Record<string, unknown>, string>;
	exParent: 'workout_id' | 'routine_id';
	setParent: 'workout_exercise_id' | 'routine_exercise_id';
}

function tables(kind: Kind): KindTables {
	return kind === 'workout'
		? {
				ex: db.workout_exercises as unknown as KindTables['ex'],
				sets: db.sets as unknown as KindTables['sets'],
				exParent: 'workout_id',
				setParent: 'workout_exercise_id'
			}
		: {
				ex: db.routine_exercises as unknown as KindTables['ex'],
				sets: db.routine_sets as unknown as KindTables['sets'],
				exParent: 'routine_id',
				setParent: 'routine_exercise_id'
			};
}

// ---------- workouts ----------

export interface WorkoutSummary extends Workout {
	exercises: string[];
}

export async function listWorkouts(): Promise<WorkoutSummary[]> {
	const workouts = live(await db.workouts.orderBy('date').reverse().toArray());
	const exercises = live(await db.workout_exercises.toArray()).sort(byPosition);
	const names = new Map<string, string[]>();
	for (const e of exercises) {
		const list = names.get(e.workout_id) ?? [];
		list.push(e.name);
		names.set(e.workout_id, list);
	}
	return workouts.map((w) => ({ ...w, exercises: names.get(w.id) ?? [] }));
}

export async function getWorkout(id: string): Promise<Workout | undefined> {
	const w = await db.workouts.get(id);
	return w && !w.deleted ? w : undefined;
}

export async function createWorkout(name = 'Workout'): Promise<string> {
	const w = fresh({ name, date: today() });
	await write(async () => {
		await db.workouts.add(w);
	});
	return w.id;
}

export async function updateWorkout(id: string, changes: Partial<Pick<Workout, 'name' | 'date'>>) {
	await write(() => patch(db.workouts, id, changes));
}

export async function deleteWorkout(id: string) {
	await write(async () => {
		await patch(db.workouts, id, { deleted: true });
		await deleteExercisesOf('workout', id);
	});
}

// ---------- routines ----------

export interface RoutineSummary extends Routine {
	exercises: string[];
}

export async function listRoutines(): Promise<RoutineSummary[]> {
	const routines = live(await db.routines.toArray()).sort((a, b) => a.name.localeCompare(b.name));
	const exercises = live(await db.routine_exercises.toArray()).sort(byPosition);
	const names = new Map<string, string[]>();
	for (const e of exercises) {
		const list = names.get(e.routine_id) ?? [];
		list.push(e.name);
		names.set(e.routine_id, list);
	}
	return routines.map((r) => ({ ...r, exercises: names.get(r.id) ?? [] }));
}

export async function getRoutine(id: string): Promise<Routine | undefined> {
	const r = await db.routines.get(id);
	return r && !r.deleted ? r : undefined;
}

export async function createRoutine(name = 'New routine'): Promise<string> {
	const r = fresh({ name });
	await write(async () => {
		await db.routines.add(r);
	});
	return r.id;
}

export async function updateRoutine(id: string, changes: Partial<Pick<Routine, 'name'>>) {
	await write(() => patch(db.routines, id, changes));
}

export async function deleteRoutine(id: string) {
	await write(async () => {
		await patch(db.routines, id, { deleted: true });
		await deleteExercisesOf('routine', id);
	});
}

/** Copy one workout's or routine's exercises and sets into another parent. */
async function copyExercises(from: Kind, fromId: string, to: Kind, toId: string) {
	const src = await loadExercises(from, fromId);
	const dst = tables(to);
	const supersets = new Map<string, string>();
	for (const e of src) {
		let superset: string | null = null;
		if (e.superset_id) {
			superset = supersets.get(e.superset_id) ?? uuid();
			supersets.set(e.superset_id, superset);
		}
		const ex = fresh({ name: e.name, position: e.position, superset_id: superset, [dst.exParent]: toId });
		await dst.ex.add(ex);
		for (const s of e.sets) {
			await dst.sets.add(
				fresh({
					position: s.position,
					reps: s.reps,
					weight_kg: s.weight_kg,
					warmup: !!s.warmup,
					rest_seconds: s.rest_seconds ?? DEFAULT_REST_SECONDS,
					[dst.setParent]: ex.id
				})
			);
		}
	}
}

export async function saveWorkoutAsRoutine(workoutId: string): Promise<string> {
	const w = await getWorkout(workoutId);
	if (!w) throw new Error('workout not found');
	const r = fresh({ name: w.name });
	await write(async () => {
		await db.routines.add(r);
		await copyExercises('workout', workoutId, 'routine', r.id);
	});
	return r.id;
}

export async function startWorkoutFromRoutine(routineId: string): Promise<string> {
	const r = await getRoutine(routineId);
	if (!r) throw new Error('routine not found');
	const w = fresh({ name: r.name, date: today() });
	await write(async () => {
		await db.workouts.add(w);
		await copyExercises('routine', routineId, 'workout', w.id);
	});
	return w.id;
}

// ---------- exercises & sets ----------

export async function loadExercises(kind: Kind, parentId: string): Promise<ExerciseView[]> {
	const t = tables(kind);
	const exercises = live(await t.ex.where(t.exParent).equals(parentId).toArray()).sort(byPosition);
	if (exercises.length === 0) return [];
	const sets = live(
		await t.sets
			.where(t.setParent)
			.anyOf(exercises.map((e) => e.id))
			.toArray()
	);
	return exercises.map((e) => ({
		id: e.id,
		name: e.name,
		position: e.position,
		superset_id: e.superset_id,
		sets: sets
			.filter((s) => s[t.setParent] === e.id)
			.sort(byPosition)
			.map(toSetView)
	}));
}

/** Group exercises into blocks: runs of 2+ adjacent exercises with one superset_id. */
export function toBlocks(exercises: ExerciseView[]): Block[] {
	const blocks: Block[] = [];
	for (const e of exercises) {
		const last = blocks.at(-1);
		const prev = last?.items.at(-1);
		if (last && prev && e.superset_id && prev.superset_id === e.superset_id) {
			last.items.push(e);
			last.superset = true;
		} else {
			blocks.push({ superset: false, items: [e] });
		}
	}
	return blocks;
}

export async function addExercise(kind: Kind, parentId: string, name: string): Promise<string> {
	const t = tables(kind);
	return write(async () => {
		const existing = await loadExercises(kind, parentId);
		const position = (existing.at(-1)?.position ?? 0) + 1;
		const ex = fresh({ name: name.trim(), position, superset_id: null, [t.exParent]: parentId });
		await t.ex.add(ex);
		await t.sets.add(fresh({
				position: 1,
				reps: 0,
				weight_kg: 0,
				warmup: false,
				rest_seconds: DEFAULT_REST_SECONDS,
				[t.setParent]: ex.id
			}));
		return ex.id;
	});
}

export async function renameExercise(kind: Kind, id: string, name: string) {
	await write(() => patch(tables(kind).ex, id, { name: name.trim() }));
}

async function deleteSetsOf(kind: Kind, exerciseIds: string[]) {
	const t = tables(kind);
	const sets = live(await t.sets.where(t.setParent).anyOf(exerciseIds).toArray());
	for (const s of sets) await patch(t.sets, s.id, { deleted: true });
}

async function deleteExercisesOf(kind: Kind, parentId: string) {
	const t = tables(kind);
	const exercises = live(await t.ex.where(t.exParent).equals(parentId).toArray());
	for (const e of exercises) await patch(t.ex, e.id, { deleted: true });
	await deleteSetsOf(
		kind,
		exercises.map((e) => e.id)
	);
}

export async function deleteExercise(kind: Kind, parentId: string, id: string) {
	await write(async () => {
		await patch(tables(kind).ex, id, { deleted: true });
		await deleteSetsOf(kind, [id]);
		await normalizeSupersets(kind, parentId);
	});
}

export async function addSet(kind: Kind, exerciseId: string) {
	const t = tables(kind);
	await write(async () => {
		const sets = live(await t.sets.where(t.setParent).equals(exerciseId).toArray()).sort(byPosition);
		const last = sets.at(-1);
		await t.sets.add(
			fresh({
				position: (last?.position ?? 0) + 1,
				reps: last?.reps ?? 0,
				weight_kg: last?.weight_kg ?? 0,
				// New sets are working sets; warm-ups are marked from the set menu.
				warmup: false,
				rest_seconds: last?.rest_seconds ?? DEFAULT_REST_SECONDS,
				[t.setParent]: exerciseId
			})
		);
	});
}

export async function updateSet(kind: Kind, id: string, changes: Partial<Pick<SetRow, 'reps' | 'weight_kg' | 'warmup' | 'rest_seconds'>>) {
	await write(() => patch(tables(kind).sets, id, changes));
}

export async function deleteSet(kind: Kind, id: string) {
	await write(() => patch(tables(kind).sets, id, { deleted: true }));
}

// ---------- reordering & supersets ----------

/** Positions strictly between lo and hi for n items (hi may be undefined = open end). */
function between(lo: number | undefined, hi: number | undefined, n: number): number[] {
	const a = lo ?? (hi !== undefined ? hi - n - 1 : 0);
	const b = hi ?? a + n + 1;
	const step = (b - a) / (n + 1);
	return Array.from({ length: n }, (_, i) => a + step * (i + 1));
}

async function setPositions(kind: Kind, items: ExerciseView[], positions: number[]) {
	const t = tables(kind);
	for (let i = 0; i < items.length; i++) {
		await patch(t.ex, items[i].id, { position: positions[i] });
	}
}

/** Move the block (single exercise or whole superset) containing exerciseId up or down one block. */
export async function moveBlock(kind: Kind, parentId: string, exerciseId: string, dir: -1 | 1) {
	await write(async () => {
		const blocks = toBlocks(await loadExercises(kind, parentId));
		const i = blocks.findIndex((b) => b.items.some((e) => e.id === exerciseId));
		const j = i + dir;
		if (i < 0 || j < 0 || j >= blocks.length) return;
		const moving = blocks[i].items;
		const target = blocks[j].items;
		const positions =
			dir === -1
				? between(blocks[j - 1]?.items.at(-1)?.position, target[0].position, moving.length)
				: between(target.at(-1)!.position, blocks[j + 1]?.items[0].position, moving.length);
		await setPositions(kind, moving, positions);
	});
}

/** Swap an exercise with its neighbour inside the same superset. */
export async function moveWithinSuperset(kind: Kind, parentId: string, exerciseId: string, dir: -1 | 1) {
	await write(async () => {
		const block = toBlocks(await loadExercises(kind, parentId)).find((b) =>
			b.items.some((e) => e.id === exerciseId)
		);
		if (!block?.superset) return;
		const i = block.items.findIndex((e) => e.id === exerciseId);
		const j = i + dir;
		if (j < 0 || j >= block.items.length) return;
		await setPositions(kind, [block.items[i], block.items[j]], [block.items[j].position, block.items[i].position]);
	});
}

/** Join an exercise to the block directly above it, forming or extending a superset. */
export async function linkToAbove(kind: Kind, parentId: string, exerciseId: string) {
	await write(async () => {
		const exercises = await loadExercises(kind, parentId);
		const i = exercises.findIndex((e) => e.id === exerciseId);
		if (i <= 0) return;
		const t = tables(kind);
		const above = exercises[i - 1];
		const me = exercises[i];
		let group = above.superset_id;
		if (!group) {
			group = uuid();
			await patch(t.ex, above.id, { superset_id: group });
		}
		// If this exercise heads a superset of its own, bring the whole group along.
		const blocks = toBlocks(exercises);
		const mine = blocks.find((b) => b.items[0].id === me.id);
		for (const e of mine?.superset ? mine.items : [me]) {
			await patch(t.ex, e.id, { superset_id: group });
		}
	});
}

/** Take an exercise out of its superset. Exercises after it stay grouped among themselves. */
export async function unlink(kind: Kind, parentId: string, exerciseId: string) {
	await write(async () => {
		const exercises = await loadExercises(kind, parentId);
		const block = toBlocks(exercises).find((b) => b.items.some((e) => e.id === exerciseId));
		if (!block?.superset) return;
		const t = tables(kind);
		const i = block.items.findIndex((e) => e.id === exerciseId);
		await patch(t.ex, exerciseId, { superset_id: null });
		const after = block.items.slice(i + 1);
		if (after.length > 0) {
			const group = after.length > 1 ? uuid() : null;
			for (const e of after) await patch(t.ex, e.id, { superset_id: group });
		}
		await normalizeSupersets(kind, parentId);
	});
}

/** Clear superset_id on exercises that no longer have a partner next to them. */
async function normalizeSupersets(kind: Kind, parentId: string) {
	const t = tables(kind);
	for (const b of toBlocks(await loadExercises(kind, parentId))) {
		const e = b.items[0];
		if (!b.superset && e.superset_id) await patch(t.ex, e.id, { superset_id: null });
	}
}

// ---------- suggestions & previous values ----------

export async function exerciseNames(): Promise<string[]> {
	const seen = new Map<string, string>();
	const rows = [
		...live(await db.workout_exercises.toArray()),
		...live(await db.routine_exercises.toArray())
	];
	for (const r of rows) {
		const key = r.name.trim().toLowerCase();
		if (key && !seen.has(key)) seen.set(key, r.name.trim());
	}
	return [...seen.values()].sort((a, b) => a.localeCompare(b));
}

/**
 * Sets from the most recent other workout (on or before this one's date) that
 * contains an exercise with this name. Keyed by exercise name (lowercased).
 */
export async function previousSets(workoutId: string): Promise<Map<string, SetView[]>> {
	const result = new Map<string, SetView[]>();
	const current = await getWorkout(workoutId);
	if (!current) return result;
	const mine = live(await db.workout_exercises.where('workout_id').equals(workoutId).toArray());
	const names = [...new Set(mine.map((e) => e.name.trim().toLowerCase()))].filter(Boolean);
	if (names.length === 0) return result;

	const workouts = new Map(live(await db.workouts.toArray()).map((w) => [w.id, w]));
	for (const name of names) {
		const candidates = live(
			await db.workout_exercises.where('name').equalsIgnoreCase(name).toArray()
		).filter((e) => {
			const w = workouts.get(e.workout_id);
			return w && w.id !== workoutId && w.date <= current.date;
		});
		candidates.sort((a, b) => {
			const wa = workouts.get(a.workout_id)!;
			const wb = workouts.get(b.workout_id)!;
			return wb.date.localeCompare(wa.date) || wb.updated_at - wa.updated_at || b.position - a.position;
		});
		for (const c of candidates) {
			const sets = live(await db.sets.where('workout_exercise_id').equals(c.id).toArray()).sort(byPosition);
			if (sets.length > 0) {
				result.set(
					name,
					sets.map(toSetView)
				);
				break;
			}
		}
	}
	return result;
}
