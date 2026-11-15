import Dexie, { type EntityTable } from 'dexie';

/** Fields every synced record has. Mirrors internal/store/schema.go. */
export interface Synced {
	id: string;
	/** Client clock, ms. Newest wins. */
	updated_at: number;
	/** Soft delete so deletes sync. */
	deleted: boolean;
	/** 1 while the change has not been accepted by the server. Local only. */
	dirty: 0 | 1;
}

export interface Workout extends Synced {
	name: string;
	/** YYYY-MM-DD */
	date: string;
	/** ms when the workout was started; missing or null on workouts from before it was recorded. */
	started_at?: number | null;
	/** ms when the workout was finished; null while it is in progress. */
	finished_at: number | null;
}

export interface Exercise extends Synced {
	name: string;
	position: number;
	/** Exercises next to each other with the same superset_id form a superset. */
	superset_id: string | null;
}

export interface WorkoutExercise extends Exercise {
	workout_id: string;
}

export interface RoutineExercise extends Exercise {
	routine_id: string;
}

export interface SetRow extends Synced {
	position: number;
	reps: number;
	weight_kg: number;
	/** Warm-up set. Missing on rows written before warm-ups existed, meaning false. */
	warmup?: boolean;
	/** Rest after this set, seconds. Missing on older rows, meaning DEFAULT_REST_SECONDS. */
	rest_seconds?: number;
}

/** Mirrors defaultRestSeconds in internal/store/schema.go. */
export const DEFAULT_REST_SECONDS = 120;

export interface WorkoutSet extends SetRow {
	workout_exercise_id: string;
	/** Checked off as performed; only done sets count. Missing on rows from before checkmarks, meaning true. */
	done?: boolean;
}

export interface RoutineSet extends SetRow {
	routine_exercise_id: string;
}

export interface Routine extends Synced {
	name: string;
}

interface Meta {
	key: string;
	value: unknown;
}

export const SYNCED_TABLES = [
	'workouts',
	'workout_exercises',
	'sets',
	'routines',
	'routine_exercises',
	'routine_sets'
] as const;
export type SyncedTable = (typeof SYNCED_TABLES)[number];

export class GymDB extends Dexie {
	workouts!: EntityTable<Workout, 'id'>;
	workout_exercises!: EntityTable<WorkoutExercise, 'id'>;
	sets!: EntityTable<WorkoutSet, 'id'>;
	routines!: EntityTable<Routine, 'id'>;
	routine_exercises!: EntityTable<RoutineExercise, 'id'>;
	routine_sets!: EntityTable<RoutineSet, 'id'>;
	meta!: EntityTable<Meta, 'key'>;

	constructor() {
		super('gym-tracker');
		this.version(1).stores({
			workouts: 'id, date, dirty',
			workout_exercises: 'id, workout_id, name, dirty',
			sets: 'id, workout_exercise_id, dirty',
			routines: 'id, dirty',
			routine_exercises: 'id, routine_id, name, dirty',
			routine_sets: 'id, routine_exercise_id, dirty',
			meta: 'key'
		});
		// Workouts from before finishing existed are history: finished at their last edit.
		// The server backfills the same way, so these rows are not marked dirty.
		this.version(2).upgrade((tx) =>
			tx
				.table('workouts')
				.toCollection()
				.modify((w: Partial<Workout>) => {
					if (w.finished_at === undefined) w.finished_at = w.updated_at;
				})
		);
		// Sets logged before checkmarks existed were performed. The server backfills
		// the same way, so these rows are not marked dirty.
		this.version(3).upgrade((tx) =>
			tx
				.table('sets')
				.toCollection()
				.modify((s: Partial<WorkoutSet>) => {
					if (s.done === undefined) s.done = true;
				})
		);
	}
}

export const db = new GymDB();
