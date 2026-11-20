/**
 * Training stats for the Profile screen, computed from local data. Only
 * finished workouts and their checked, non-warm-up sets count, the same rule
 * as workout totals and the webhook.
 */
import { catalogExercise } from './catalog';
import { addDays, parseIso, weekStart } from './dates';
import { db } from './db';

export interface HistorySet {
	reps: number;
	weight_kg: number;
	/** Length of a timed set, seconds; 0 for sets counted in reps. */
	duration_seconds: number;
}

/** One exercise as performed in one finished workout. */
export interface Performance {
	workoutId: string;
	workoutName: string;
	/** YYYY-MM-DD */
	date: string;
	/** The exercise's name. */
	name: string;
	/** The exercise's catalogue id. */
	key: string;
	/** Logged by time instead of reps. */
	timed: boolean;
	sets: HistorySet[];
}

export interface History {
	/** Dates of finished workouts, one entry per workout. */
	workoutDates: string[];
	/** Oldest first. */
	performances: Performance[];
}

export async function loadHistory(): Promise<History> {
	const workouts = (await db.workouts.toArray()).filter((w) => !w.deleted && w.finished_at != null);
	const byId = new Map(workouts.map((w) => [w.id, w]));
	const exercises = (await db.workout_exercises.toArray()).filter(
		(e) => !e.deleted && byId.has(e.workout_id)
	);
	const setsByExercise = new Map<string, HistorySet[]>();
	for (const s of await db.sets.toArray()) {
		// A set with no reps or time was not performed, checked or not.
		const duration_seconds = s.duration_seconds;
		if (s.deleted || s.warmup || !s.done || (s.reps <= 0 && duration_seconds <= 0)) continue;
		const list = setsByExercise.get(s.workout_exercise_id) ?? [];
		list.push({ reps: s.reps, weight_kg: s.weight_kg, duration_seconds });
		setsByExercise.set(s.workout_exercise_id, list);
	}
	const performances: Performance[] = [];
	for (const e of exercises) {
		const sets = setsByExercise.get(e.id);
		const exercise = catalogExercise(e.exercise_id);
		if (!sets?.length || !exercise) continue;
		const w = byId.get(e.workout_id)!;
		performances.push({
			workoutId: w.id,
			workoutName: w.name,
			date: w.date,
			name: exercise.name,
			key: exercise.id,
			timed: exercise.measure === 'time',
			sets
		});
	}
	performances.sort((a, b) => a.date.localeCompare(b.date));
	return { workoutDates: workouts.map((w) => w.date), performances };
}

// ---------- numbers ----------

/**
 * Estimated one-rep max (Epley). Only sets of 1–12 reps with weight give an
 * estimate; beyond that the formula stops being meaningful.
 */
export function e1rm(s: HistorySet): number {
	if (s.weight_kg <= 0 || s.reps < 1 || s.reps > 12) return 0;
	return s.reps === 1 ? s.weight_kg : s.weight_kg * (1 + s.reps / 30);
}

// ---------- summary ----------

export interface Summary {
	thisWeek: number;
	thisMonth: number;
	/**
	 * Workouts per week over the finished weeks of the last 12, counted from the first
	 * workout so a new user isn't averaged over weeks before they started. Null until
	 * a full week has passed, since this week alone would just repeat `thisWeek`.
	 */
	avgPerWeek: number | null;
	/**
	 * Weeks in a row that met the weekly goal (any workout when there is none), counting
	 * back from this week, or from last week while this one has not met it yet.
	 */
	streakWeeks: number;
}

export function summary(h: History, today: string, goal = 0): Summary {
	const week = weekStart(today);
	const month = today.slice(0, 7);
	const perWeek = new Map<string, number>();
	for (const d of h.workoutDates) perWeek.set(weekStart(d), (perWeek.get(weekStart(d)) ?? 0) + 1);
	const met = (start: string) => (perWeek.get(start) ?? 0) >= Math.max(1, goal);
	let cursor = met(week) ? week : addDays(week, -7);
	let streakWeeks = 0;
	while (met(cursor)) {
		streakWeeks++;
		cursor = addDays(cursor, -7);
	}
	const first = h.workoutDates.reduce<string | null>((a, d) => (a == null || d < a ? d : a), null);
	let avgPerWeek: number | null = null;
	if (first) {
		const from = [weekStart(first), addDays(week, -7 * 12)].sort()[1];
		const finished = h.workoutDates.filter((d) => d >= from && d < week).length;
		const weeks = Math.round((parseIso(week).getTime() - parseIso(from).getTime()) / (7 * 86_400_000));
		if (weeks > 0) avgPerWeek = finished / weeks;
	}
	return {
		thisWeek: h.workoutDates.filter((d) => d >= week && d <= today).length,
		thisMonth: h.workoutDates.filter((d) => d.startsWith(month)).length,
		avgPerWeek,
		streakWeeks
	};
}

/**
 * Whether this week's goal is at risk: the workouts still needed take every day
 * left in the week, today included, or more.
 */
export function behindGoal(thisWeek: number, goal: number, today: string): boolean {
	const needed = goal - thisWeek;
	const daysLeft = 7 - ((parseIso(today).getDay() + 6) % 7);
	return needed > 0 && needed >= daysLeft;
}

// ---------- weekly workouts ----------

export interface Week {
	/** Monday, YYYY-MM-DD */
	start: string;
	workouts: number;
}

/** The last `count` weeks up to and including this one, oldest first. */
export function weekly(h: History, today: string, count = 12): Week[] {
	const first = addDays(weekStart(today), -7 * (count - 1));
	const weeks: Week[] = Array.from({ length: count }, (_, i) => ({
		start: addDays(first, 7 * i),
		workouts: 0
	}));
	const index = new Map(weeks.map((w, i) => [w.start, i]));
	for (const d of h.workoutDates) {
		const i = index.get(weekStart(d));
		if (i !== undefined) weeks[i].workouts++;
	}
	return weeks;
}

// ---------- records ----------

export interface Best {
	value: number;
	/** The set that holds it. */
	set: HistorySet;
	date: string;
	workoutId: string;
	/** Set after earlier history of this exercise, i.e. it beat a previous best. */
	improved: boolean;
}

export interface ExerciseStats {
	key: string;
	/** Most recently used spelling. */
	name: string;
	sessions: number;
	lastDate: string;
	/** Logged by time instead of reps. */
	timed: boolean;
	heaviest: Best | null;
	bestE1rm: Best | null;
	mostReps: Best | null;
	/** Longest timed set. */
	longest: Best | null;
}

function better(
	current: Best | null,
	value: number,
	p: Performance,
	set: HistorySet,
	seen: boolean
): Best | null {
	if (value <= 0 || (current && value <= current.value)) return current;
	return { value, set, date: p.date, workoutId: p.workoutId, improved: seen };
}

type BestKey = 'heaviest' | 'bestE1rm' | 'mostReps' | 'longest';

/** What each best measures in a set. */
const BEST_VALUE: Record<BestKey, (s: HistorySet) => number> = {
	// Heavier weight wins; at the same weight, more reps (or a longer timed set).
	heaviest: (s) => s.weight_kg + s.reps / 1000 + s.duration_seconds / 1e6,
	bestE1rm: e1rm,
	mostReps: (s) => s.reps,
	longest: (s) => s.duration_seconds
};

/** Every exercise with its bests, alphabetical by name. */
export function exerciseStats(h: History): ExerciseStats[] {
	const map = new Map<string, ExerciseStats>();
	for (const p of h.performances) {
		let x = map.get(p.key);
		const seen = !!x;
		if (!x) {
			x = {
				key: p.key,
				name: p.name,
				sessions: 0,
				lastDate: p.date,
				timed: p.timed,
				heaviest: null,
				bestE1rm: null,
				mostReps: null,
				longest: null
			};
			map.set(p.key, x);
		}
		x.sessions++;
		x.lastDate = p.date;
		x.name = p.name;
		for (const [k, value] of Object.entries(BEST_VALUE) as [BestKey, (s: HistorySet) => number][]) {
			// The best within this session first, so a session that beats a record twice counts once.
			let session: Best | null = null;
			for (const s of p.sets) session = better(session, value(s), p, s, seen);
			const current = x[k];
			if (session && (!current || session.value > current.value)) x[k] = session;
		}
	}
	return [...map.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export interface RecentRecord {
	key: string;
	name: string;
	kind: 'heaviest' | 'e1rm' | 'reps' | 'time';
	record: Best;
}

/**
 * Records that beat an earlier best and were set within `days` of today,
 * newest first. A heavier set beats an e1RM record set the same day, so each
 * exercise shows at most one record per day.
 */
export function recentRecords(stats: ExerciseStats[], today: string, days = 30): RecentRecord[] {
	const since = addDays(today, -days);
	const out: RecentRecord[] = [];
	for (const x of stats) {
		const candidates: RecentRecord[] = [];
		const add = (kind: RecentRecord['kind'], record: Best | null) => {
			if (record) candidates.push({ key: x.key, name: x.name, kind, record });
		};
		add('heaviest', x.heaviest);
		add('e1rm', x.bestE1rm);
		// Reps or time only count as a record for exercises done without weight.
		if (!x.heaviest?.set.weight_kg) add(x.timed ? 'time' : 'reps', x.timed ? x.longest : x.mostReps);
		const shown = new Set<string>();
		for (const c of candidates) {
			if (!c.record.improved || c.record.date < since || shown.has(c.record.date)) continue;
			shown.add(c.record.date);
			out.push(c);
		}
	}
	return out.sort((a, b) => b.record.date.localeCompare(a.record.date) || a.name.localeCompare(b.name));
}

// ---------- one exercise ----------

export interface Session {
	workoutId: string;
	workoutName: string;
	date: string;
	sets: HistorySet[];
	bestE1rm: number;
	topWeight: number;
}

/** Every session of one exercise, newest first. */
export function exerciseSessions(h: History, key: string): Session[] {
	return h.performances
		.filter((p) => p.key === key)
		.map((p) => ({
			workoutId: p.workoutId,
			workoutName: p.workoutName,
			date: p.date,
			sets: p.sets,
			bestE1rm: Math.max(0, ...p.sets.map(e1rm)),
			topWeight: Math.max(0, ...p.sets.map((s) => s.weight_kg))
		}))
		.reverse();
}

/** A value on a date, for charts. */
export interface Point {
	/** YYYY-MM-DD */
	date: string;
	value: number;
}
