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
	/** ms when the workout was started. */
	started_at: number | null;
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
	/** Warm-up set; left out of totals. */
	warmup: boolean;
	/** Rest after this set, seconds. */
	rest_seconds: number;
	/**
	 * Length of a timed set (e.g. a dead hang), seconds; 0 for sets counted in
	 * reps. Rows from before timed sets may lack it, so read it with `?? 0`.
	 */
	duration_seconds?: number;
}

/** Rest given to new sets, seconds. */
export const DEFAULT_REST_SECONDS = 120;

export interface WorkoutSet extends SetRow {
	workout_exercise_id: string;
	/** Checked off as performed; only done sets count. */
	done: boolean;
}

export interface RoutineSet extends SetRow {
	routine_exercise_id: string;
}

export interface Routine extends Synced {
	name: string;
}

/** One weigh-in per date, with the date as id. Workouts use the latest one on or before their date. */
export interface BodyWeight extends Synced {
	/** YYYY-MM-DD */
	date: string;
	weight_kg: number;
}

/** Settings for an exercise, with exerciseKey(name) as id. */
export interface ExerciseSetting extends Synced {
	name: string;
	/** Share of body weight each rep moves, 0–100 (e.g. 100 for pull-ups). */
	bodyweight_pct: number;
	/** Logged by time (duration_seconds) instead of reps. Missing on rows from before it existed. */
	timed?: boolean;
}

/** User settings: one row with id SETTINGS_ID, so every device edits the same row. */
export interface Settings extends Synced {
	/** Workouts per week to aim for, 0 for no goal. */
	weekly_goal: number;
}

/** Mirrors SettingsID in internal/store/schema.go. */
export const SETTINGS_ID = 'user';

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
	'routine_sets',
	'body_weights',
	'exercise_settings',
	'settings'
] as const;
export type SyncedTable = (typeof SYNCED_TABLES)[number];

export class GymDB extends Dexie {
	workouts!: EntityTable<Workout, 'id'>;
	workout_exercises!: EntityTable<WorkoutExercise, 'id'>;
	sets!: EntityTable<WorkoutSet, 'id'>;
	routines!: EntityTable<Routine, 'id'>;
	routine_exercises!: EntityTable<RoutineExercise, 'id'>;
	routine_sets!: EntityTable<RoutineSet, 'id'>;
	body_weights!: EntityTable<BodyWeight, 'id'>;
	exercise_settings!: EntityTable<ExerciseSetting, 'id'>;
	settings!: EntityTable<Settings, 'id'>;
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
			body_weights: 'id, dirty',
			exercise_settings: 'id, dirty',
			settings: 'id, dirty',
			meta: 'key'
		});
	}
}

export const db = new GymDB();
