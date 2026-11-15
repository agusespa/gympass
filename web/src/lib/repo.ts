/**
 * All reads and writes of local data. Every write stamps updated_at, marks the
 * row dirty and notifies listeners (the sync loop).
 */
import type { Table } from 'dexie';
import {
	db,
	DEFAULT_REST_SECONDS,
	SETTINGS_ID,
	type Exercise,
	type Routine,
	type SetRow,
	type Settings,
	type Synced,
	type BodyWeight,
	type Workout
} from './db';
import { exerciseKey, trimName } from './exercise-key';
import { MAX_REPS, MAX_TEXT_LEN, MAX_WEIGHT_KG } from './limits';

export type Kind = 'workout' | 'routine';

export interface SetView {
	id: string;
	position: number;
	reps: number;
	weight_kg: number;
	warmup: boolean;
	rest_seconds: number;
	/** Checked off as performed. Workout sets only; routine sets are always true. */
	done: boolean;
}

/** Routine sets have no done flag: they are plans, so they count as done. */
function toSetView(s: SetRow & { done?: boolean }): SetView {
	return {
		id: s.id,
		position: s.position,
		reps: s.reps,
		weight_kg: s.weight_kg,
		warmup: s.warmup,
		rest_seconds: s.rest_seconds,
		done: s.done ?? true
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

/** A user-typed name as the server accepts it: trimmed, at most MAX_TEXT_LEN code points. */
export function cleanName(name: string): string {
	return Array.from(trimName(name)).slice(0, MAX_TEXT_LEN).join('');
}

function clamp(n: number, max: number): number {
	return Number.isFinite(n) ? Math.min(max, Math.max(0, n)) : 0;
}

function fresh<T extends object>(row: T): T & Synced {
	return keyed(uuid(), row);
}

/**
 * A row whose id comes from what it describes (a date, an exercise, the
 * settings singleton), so every device writes the same row. Putting it
 * replaces any earlier version, deleted or not.
 */
function keyed<T extends object>(id: string, row: T): T & Synced {
	return { ...row, id, updated_at: stamp(), deleted: false, dirty: 1 } as T & Synced;
}

const allTables = () => [
	db.workouts,
	db.workout_exercises,
	db.sets,
	db.routines,
	db.routine_exercises,
	db.routine_sets,
	db.body_weights,
	db.exercise_settings,
	db.settings
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

/** One row by id, unless it is missing or soft-deleted. */
async function getLive<T extends Synced>(
	table: { get(id: string): PromiseLike<T | undefined> },
	id: string
): Promise<T | undefined> {
	const row = await table.get(id);
	return row && !row.deleted ? row : undefined;
}

/** Exercise names per parent id, in exercise order. */
function namesByParent<E extends Exercise>(
	exercises: E[],
	parentOf: (e: E) => string
): Map<string, string[]> {
	const names = new Map<string, string[]>();
	for (const e of exercises.sort(byPosition)) {
		const list = names.get(parentOf(e)) ?? [];
		list.push(e.name);
		names.set(parentOf(e), list);
	}
	return names;
}

/** A new set row with defaults; values from `like` (e.g. the set it copies) override them. */
function newSet(
	parentKey: KindTables['setParent'],
	parentId: string,
	like?: Partial<Omit<SetView, 'id'>>
): SetRow & Synced & Record<string, unknown> {
	const { done = false, ...rest } = like ?? {};
	return fresh({
		position: 1,
		reps: 0,
		weight_kg: 0,
		warmup: false,
		rest_seconds: DEFAULT_REST_SECONDS,
		...rest,
		// Only workout sets get checked off; routine sets are plans.
		...(parentKey === 'workout_exercise_id' ? { done } : {}),
		[parentKey]: parentId
	}) as SetRow & Synced & Record<string, unknown>;
}

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
	const names = namesByParent(live(await db.workout_exercises.toArray()), (e) => e.workout_id);
	return workouts.map((w) => ({ ...w, exercises: names.get(w.id) ?? [] }));
}

/** Workouts started but not finished yet, newest first. */
export async function listRunningWorkouts(): Promise<WorkoutSummary[]> {
	return (await listWorkouts()).filter((w) => w.finished_at == null);
}

export function getWorkout(id: string): Promise<Workout | undefined> {
	return getLive<Workout>(db.workouts, id);
}

async function isFinished(workoutId: string): Promise<boolean> {
	return (await db.workouts.get(workoutId))?.finished_at != null;
}

export async function createWorkout(name = 'Workout'): Promise<string> {
	const w = fresh({ name: cleanName(name), date: today(), started_at: Date.now(), finished_at: null });
	await write(async () => {
		await db.workouts.add(w);
	});
	return w.id;
}

export async function updateWorkout(id: string, changes: Partial<Pick<Workout, 'name' | 'date'>>) {
	if (changes.name !== undefined) changes = { ...changes, name: cleanName(changes.name) };
	await write(() => patch(db.workouts, id, changes));
}

export async function finishWorkout(id: string) {
	await write(() => patch(db.workouts, id, { finished_at: Date.now() }));
}

/** Undo finishWorkout: the workout is in progress again. */
export async function resumeWorkout(id: string) {
	await write(() => patch(db.workouts, id, { finished_at: null }));
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
	const names = namesByParent(live(await db.routine_exercises.toArray()), (e) => e.routine_id);
	return routines.map((r) => ({ ...r, exercises: names.get(r.id) ?? [] }));
}

export function getRoutine(id: string): Promise<Routine | undefined> {
	return getLive<Routine>(db.routines, id);
}

export async function createRoutine(name = 'New routine'): Promise<string> {
	const r = fresh({ name: cleanName(name) });
	await write(async () => {
		await db.routines.add(r);
	});
	return r.id;
}

export async function updateRoutine(id: string, changes: Partial<Pick<Routine, 'name'>>) {
	if (changes.name !== undefined) changes = { ...changes, name: cleanName(changes.name) };
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
		// Copied sets are targets to work through, so a new workout starts with none checked off.
		for (const { id: _id, done: _done, ...set } of e.sets) {
			await dst.sets.add(newSet(dst.setParent, ex.id, set));
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
	const w = fresh({ name: r.name, date: today(), started_at: Date.now(), finished_at: null });
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
		const ex = fresh({ name: cleanName(name), position, superset_id: null, [t.exParent]: parentId });
		await t.ex.add(ex);
		// A set added to a running workout is still to do; one added to history already happened.
		const done = kind === 'workout' && (await isFinished(parentId));
		// Start from what was done last time, so only the numbers need changing.
		const before = kind === 'workout' ? (await getWorkout(parentId))?.date : undefined;
		const last = await lastSetsFor(ex.name, { exclude: parentId, before });
		const sets = last.length > 0 ? last : [undefined];
		for (const [i, s] of sets.entries()) {
			const isLast = i === sets.length - 1;
			await t.sets.add(
				newSet(t.setParent, ex.id, {
					position: i + 1,
					...(s && { reps: s.reps, weight_kg: s.weight_kg, warmup: s.warmup }),
					// The last set gets no rest; the others keep theirs (or the default).
					rest_seconds: isLast ? 0 : s?.rest_seconds || DEFAULT_REST_SECONDS,
					done
				})
			);
		}
		return ex.id;
	});
}

export async function renameExercise(kind: Kind, id: string, name: string) {
	await write(() => patch(tables(kind).ex, id, { name: cleanName(name) }));
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
		// The new set is the last one, so it gets no rest. The old last set had none by default;
		// it now needs one, taken from the set before it (or the default).
		if (last && last.rest_seconds === 0) {
			const prev = sets.at(-2)?.rest_seconds ?? DEFAULT_REST_SECONDS;
			await patch(t.sets, last.id, { rest_seconds: prev || DEFAULT_REST_SECONDS });
		}
		// A set added to a running workout is still to do; one added to history already happened.
		let done = false;
		if (kind === 'workout') {
			const ex = await db.workout_exercises.get(exerciseId);
			done = !!ex && (await isFinished(ex.workout_id));
		}
		// Copy the last set's numbers; new sets are working sets (warm-ups are marked from the set menu).
		await t.sets.add(
			newSet(
				t.setParent,
				exerciseId,
				last
					? { position: last.position + 1, reps: last.reps, weight_kg: last.weight_kg, rest_seconds: 0, done }
					: { rest_seconds: 0, done }
			)
		);
	});
}

export async function updateSet(
	kind: Kind,
	id: string,
	changes: Partial<Pick<SetRow, 'reps' | 'weight_kg' | 'warmup' | 'rest_seconds'> & { done: boolean }>
) {
	const bounded = { ...changes };
	if (bounded.reps !== undefined) bounded.reps = Math.round(clamp(bounded.reps, MAX_REPS));
	if (bounded.weight_kg !== undefined) bounded.weight_kg = clamp(bounded.weight_kg, MAX_WEIGHT_KG);
	await write(() => patch(tables(kind).sets, id, bounded));
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
		await setPositions(
			kind,
			[block.items[i], block.items[j]],
			[block.items[j].position, block.items[i].position]
		);
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
	const rows = [...live(await db.workout_exercises.toArray()), ...live(await db.routine_exercises.toArray())];
	for (const r of rows) {
		const key = exerciseKey(r.name);
		if (key && !seen.has(key)) seen.set(key, trimName(r.name));
	}
	return [...seen.values()].sort((a, b) => a.localeCompare(b));
}

/**
 * The sets of the last time an exercise was done: the most recent workout
 * (other than `exclude`, on or before `before`) with done sets of it, else the
 * most recently edited routine (other than `exclude`) that has it. Empty when
 * neither exists (or, with `workoutsOnly`, no workout has it). Warm-ups
 * included; from workouts, only performed sets.
 */
async function lastSetsFor(
	name: string,
	opts: { exclude?: string; before?: string; workoutsOnly?: boolean } = {}
): Promise<SetView[]> {
	const key = exerciseKey(name);
	if (!key) return [];
	const workouts = new Map(live(await db.workouts.toArray()).map((w) => [w.id, w]));
	const candidates = live(await db.workout_exercises.where('name').equalsIgnoreCase(key).toArray()).filter(
		(e) => {
			const w = workouts.get(e.workout_id);
			return w && w.id !== opts.exclude && (opts.before === undefined || w.date <= opts.before);
		}
	);
	candidates.sort((a, b) => {
		const wa = workouts.get(a.workout_id)!;
		const wb = workouts.get(b.workout_id)!;
		return wb.date.localeCompare(wa.date) || wb.updated_at - wa.updated_at || b.position - a.position;
	});
	for (const c of candidates) {
		// Only what was actually performed; unchecked sets were plans that didn't happen.
		const sets = live(await db.sets.where('workout_exercise_id').equals(c.id).toArray())
			.filter((s) => s.done)
			.sort(byPosition);
		if (sets.length > 0) return sets.map(toSetView);
	}
	if (opts.workoutsOnly) return [];

	const routines = new Map(live(await db.routines.toArray()).map((r) => [r.id, r]));
	const planned = live(await db.routine_exercises.where('name').equalsIgnoreCase(key).toArray())
		.filter((e) => routines.has(e.routine_id) && e.routine_id !== opts.exclude)
		.sort((a, b) => b.updated_at - a.updated_at);
	for (const p of planned) {
		const sets = live(await db.routine_sets.where('routine_exercise_id').equals(p.id).toArray()).sort(
			byPosition
		);
		if (sets.length > 0) return sets.map(toSetView);
	}
	return [];
}

/**
 * Sets from the most recent other workout (on or before this one's date) that
 * contains an exercise with this name. Keyed by exercise name (lowercased).
 * Workouts only: the hint shows what was done, never a routine's plan.
 */
export async function previousSets(workoutId: string): Promise<Map<string, SetView[]>> {
	const result = new Map<string, SetView[]>();
	const current = await getWorkout(workoutId);
	if (!current) return result;
	const mine = live(await db.workout_exercises.where('workout_id').equals(workoutId).toArray());
	const names = [...new Set(mine.map((e) => exerciseKey(e.name)))].filter(Boolean);
	for (const name of names) {
		const sets = await lastSetsFor(name, { exclude: workoutId, before: current.date, workoutsOnly: true });
		if (sets.length > 0) result.set(name, sets);
	}
	return result;
}

// ---------- body weight ----------

/** Weigh-ins, newest date first. */
export async function listBodyWeights(): Promise<BodyWeight[]> {
	return live(await db.body_weights.toArray()).sort((a, b) => b.date.localeCompare(a.date));
}

const BODY_WEIGHT_CARD_KEY = 'body_weight_card_dismissed';

/** Whether Profile should ask for a body weight: none logged yet and the card not dismissed on this phone. */
export async function showBodyWeightCard(): Promise<boolean> {
	if ((await db.meta.get(BODY_WEIGHT_CARD_KEY))?.value) return false;
	return !(await db.body_weights.filter((w) => !w.deleted).first());
}

/** Hide the body weight card on this phone. Local only: it is not synced. */
export async function dismissBodyWeightCard() {
	await db.meta.put({ key: BODY_WEIGHT_CARD_KEY, value: true });
}

/** Record the body weight for a date, replacing that date's weigh-in. */
export function logBodyWeight(date: string, weight_kg: number) {
	return write(() => db.body_weights.put(keyed(date, { date, weight_kg: clamp(weight_kg, MAX_WEIGHT_KG) })));
}

export function deleteBodyWeight(date: string) {
	return write(() => patch(db.body_weights, date, { deleted: true }));
}

/** Set the share of body weight each rep of an exercise moves, 0–100. */
export function setBodyweightPct(name: string, pct: number) {
	// The server derives the id from the stored name, so both come from one cleaned value.
	const clean = cleanName(name);
	const key = exerciseKey(clean);
	if (!key) return Promise.resolve();
	const bodyweight_pct = Math.round(clamp(pct, 100));
	return write(() => db.exercise_settings.put(keyed(key, { name: clean, bodyweight_pct })));
}

// ---------- settings ----------

/** Mirrors maxWeeklyGoal in internal/store/schema.go. */
export const MAX_WEEKLY_GOAL = 7;

/** Workouts per week to aim for, 0 for no goal. */
export async function getWeeklyGoal(): Promise<number> {
	return (await getLive<Settings>(db.settings, SETTINGS_ID))?.weekly_goal ?? 0;
}

export function setWeeklyGoal(goal: number) {
	const weekly_goal = Math.max(0, Math.min(MAX_WEEKLY_GOAL, Math.round(goal)));
	return write(() => db.settings.put(keyed(SETTINGS_ID, { weekly_goal })));
}
