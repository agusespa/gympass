/**
 * Training stats for the Profile screen, computed from local data. Only
 * finished workouts and their checked, non-warm-up sets count, the same rule
 * as workout totals and the webhook.
 *
 * Volume counts body weight for exercises that move it: each rep adds
 * weight_kg + body weight × bodyweight_pct / 100, the same as the server
 * (internal/webhook FromSnapshot).
 */
import { db } from './db';
import { exerciseKey } from './exercise-key';

export interface HistorySet {
	reps: number;
	weight_kg: number;
}

/** One exercise as performed in one finished workout. */
export interface Performance {
	workoutId: string;
	workoutName: string;
	/** YYYY-MM-DD */
	date: string;
	/** Exercise name as typed in this workout. */
	name: string;
	/** Lowercased, trimmed name: exercises with the same key are the same exercise. */
	key: string;
	/** Body weight each rep moves on top of weight_kg, kg (0 for most exercises). */
	body: number;
	sets: HistorySet[];
}

export interface History {
	/** Dates of finished workouts, one entry per workout. */
	workoutDates: string[];
	/** Oldest first. */
	performances: Performance[];
}

export { exerciseKey };

// ---------- body weight ----------

export interface BodyContext {
	/** One weigh-in per date, oldest first. */
	weights: { date: string; weight_kg: number }[];
	/** bodyweight_pct by exercise key; missing means 0. */
	shares: Map<string, number>;
}

/** Weigh-ins and bodyweight shares. */
export async function loadBodyContext(): Promise<BodyContext> {
	const weights = (await db.body_weights.toArray())
		.filter((w) => !w.deleted)
		.sort((a, b) => a.date.localeCompare(b.date))
		.map(({ date, weight_kg }) => ({ date, weight_kg }));
	const shares = new Map(
		(await db.exercise_settings.toArray())
			.filter((x) => !x.deleted && x.bodyweight_pct > 0)
			.map((x) => [x.id, x.bodyweight_pct])
	);
	return { weights, shares };
}

/**
 * Body weight on a date: the latest weigh-in on or before it, or the first
 * one for earlier dates. Null when nothing was ever logged.
 */
export function bodyWeightOn(weights: BodyContext['weights'], date: string): number | null {
	let found: number | null = weights[0]?.weight_kg ?? null;
	for (const w of weights) {
		if (w.date > date) break;
		found = w.weight_kg;
	}
	return found;
}

/** Body weight each rep of this exercise moves on that date, kg. */
export function bodyLoad(ctx: BodyContext, key: string, date: string): number {
	const pct = ctx.shares.get(key) ?? 0;
	if (pct <= 0) return 0;
	return ((bodyWeightOn(ctx.weights, date) ?? 0) * pct) / 100;
}

export async function loadHistory(): Promise<History> {
	const workouts = (await db.workouts.toArray()).filter((w) => !w.deleted && w.finished_at != null);
	const byId = new Map(workouts.map((w) => [w.id, w]));
	const exercises = (await db.workout_exercises.toArray()).filter(
		(e) => !e.deleted && byId.has(e.workout_id)
	);
	const setsByExercise = new Map<string, HistorySet[]>();
	for (const s of await db.sets.toArray()) {
		// A set with no reps was not performed, checked or not.
		if (s.deleted || s.warmup || !s.done || s.reps <= 0) continue;
		const list = setsByExercise.get(s.workout_exercise_id) ?? [];
		list.push({ reps: s.reps, weight_kg: s.weight_kg });
		setsByExercise.set(s.workout_exercise_id, list);
	}
	const body = await loadBodyContext();
	const performances: Performance[] = [];
	for (const e of exercises) {
		const sets = setsByExercise.get(e.id);
		const key = exerciseKey(e.name);
		if (!sets?.length || !key) continue;
		const w = byId.get(e.workout_id)!;
		performances.push({
			workoutId: w.id,
			workoutName: w.name,
			date: w.date,
			name: e.name.trim(),
			key,
			body: bodyLoad(body, key, w.date),
			sets
		});
	}
	performances.sort((a, b) => a.date.localeCompare(b.date));
	return { workoutDates: workouts.map((w) => w.date), performances };
}

// ---------- dates ----------

function parse(iso: string): Date {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d);
}

export function isoDate(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** The Monday of the week holding iso. */
export function weekStart(iso: string): string {
	const d = parse(iso);
	d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
	return isoDate(d);
}

function addDays(iso: string, days: number): string {
	const d = parse(iso);
	d.setDate(d.getDate() + days);
	return isoDate(d);
}

// ---------- numbers ----------

/** Weight × reps, where each rep also moves `body` kg of body weight. Assisted (negative) loads stop at 0. */
export function volume(sets: HistorySet[], body = 0): number {
	return sets.reduce((n, s) => n + Math.max(0, s.weight_kg + body) * s.reps, 0);
}

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
	totalVolume: number;
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
		const weeks = Math.round((parse(week).getTime() - parse(from).getTime()) / (7 * 86_400_000));
		if (weeks > 0) avgPerWeek = finished / weeks;
	}
	return {
		thisWeek: h.workoutDates.filter((d) => d >= week && d <= today).length,
		thisMonth: h.workoutDates.filter((d) => d.startsWith(month)).length,
		avgPerWeek,
		streakWeeks,
		totalVolume: h.performances.reduce((n, p) => n + volume(p.sets, p.body), 0)
	};
}

/**
 * Whether this week's goal is at risk: the workouts still needed take every day
 * left in the week, today included, or more.
 */
export function behindGoal(thisWeek: number, goal: number, today: string): boolean {
	const needed = goal - thisWeek;
	const daysLeft = 7 - ((parse(today).getDay() + 6) % 7);
	return needed > 0 && needed >= daysLeft;
}

// ---------- weekly volume ----------

export interface Week {
	/** Monday, YYYY-MM-DD */
	start: string;
	volume: number;
	workouts: number;
}

/** The last `count` weeks up to and including this one, oldest first. */
export function weekly(h: History, today: string, count = 12): Week[] {
	const first = addDays(weekStart(today), -7 * (count - 1));
	const weeks: Week[] = Array.from({ length: count }, (_, i) => ({
		start: addDays(first, 7 * i),
		volume: 0,
		workouts: 0
	}));
	const index = new Map(weeks.map((w, i) => [w.start, i]));
	for (const d of h.workoutDates) {
		const i = index.get(weekStart(d));
		if (i !== undefined) weeks[i].workouts++;
	}
	for (const p of h.performances) {
		const i = index.get(weekStart(p.date));
		if (i !== undefined) weeks[i].volume += volume(p.sets, p.body);
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
	heaviest: Best | null;
	bestE1rm: Best | null;
	mostReps: Best | null;
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
				heaviest: null,
				bestE1rm: null,
				mostReps: null
			};
			map.set(p.key, x);
		}
		x.sessions++;
		x.lastDate = p.date;
		x.name = p.name;
		// Bests within this session first, so a session that beats a record twice counts once.
		let heaviest: Best | null = null;
		let bestE1rm: Best | null = null;
		let mostReps: Best | null = null;
		for (const s of p.sets) {
			// Heavier weight wins; at the same weight, more reps.
			heaviest = better(heaviest, s.weight_kg + s.reps / 1000, p, s, seen);
			bestE1rm = better(bestE1rm, e1rm(s), p, s, seen);
			mostReps = better(mostReps, s.reps, p, s, seen);
		}
		if (heaviest && (!x.heaviest || heaviest.value > x.heaviest.value)) x.heaviest = heaviest;
		if (bestE1rm && (!x.bestE1rm || bestE1rm.value > x.bestE1rm.value)) x.bestE1rm = bestE1rm;
		if (mostReps && (!x.mostReps || mostReps.value > x.mostReps.value)) x.mostReps = mostReps;
	}
	return [...map.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export interface RecentRecord {
	key: string;
	name: string;
	kind: 'heaviest' | 'e1rm' | 'reps';
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
		if (x.heaviest) candidates.push({ key: x.key, name: x.name, kind: 'heaviest', record: x.heaviest });
		if (x.bestE1rm) candidates.push({ key: x.key, name: x.name, kind: 'e1rm', record: x.bestE1rm });
		if (x.mostReps && !x.heaviest?.set.weight_kg)
			candidates.push({ key: x.key, name: x.name, kind: 'reps', record: x.mostReps });
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
	volume: number;
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
			topWeight: Math.max(0, ...p.sets.map((s) => s.weight_kg)),
			volume: volume(p.sets, p.body)
		}))
		.reverse();
}

/** A value on a date, for charts. */
export interface Point {
	/** YYYY-MM-DD */
	date: string;
	value: number;
}
