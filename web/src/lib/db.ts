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
	started_at: number;
	/** ms when the workout was finished; null while it is in progress. */
	finished_at: number | null;
}

/** An exercise's place in a workout or routine. */
export interface ExerciseEntry extends Synced {
	/** Id of a catalogue entry (catalog.ts). */
	exercise_id: string;
	position: number;
	/** Entries next to each other with the same superset_id form a superset. */
	superset_id: string | null;
}

export interface WorkoutExercise extends ExerciseEntry {
	workout_id: string;
}

export interface RoutineExercise extends ExerciseEntry {
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
	/** Length of a timed set (e.g. a dead hang), seconds; 0 for sets counted in reps. */
	duration_seconds: number;
	/**
	 * Target rep range, e.g. 8 to 12; both 0 when there is none. A routine set
	 * plans reps = rep_min; a workout set copies the range from its routine
	 * and records the reps actually done.
	 */
	rep_min: number;
	rep_max: number;
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
	settings!: EntityTable<Settings, 'id'>;
	meta!: EntityTable<Meta, 'key'>;

	constructor() {
		super('gym-tracker');
		this.version(1).stores({
			workouts: 'id, date, dirty',
			workout_exercises: 'id, workout_id, exercise_id, dirty',
			sets: 'id, workout_exercise_id, dirty',
			routines: 'id, dirty',
			routine_exercises: 'id, routine_id, exercise_id, dirty',
			routine_sets: 'id, routine_exercise_id, dirty',
			settings: 'id, dirty',
			meta: 'key'
		});
	}
}

export const db = new GymDB();
