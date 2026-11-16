import type { Equipment } from './catalog';
import type { SetView } from './repo';

/** Smallest weight increase to suggest per equipment, kg; 0 for no suggestion. */
export const WEIGHT_STEP_KG: Record<Equipment, number> = {
	barbell: 2.5,
	dumbbell: 2,
	machine: 2.5,
	cable: 2.5,
	kettlebell: 2,
	bodyweight: 0
};

/** One step more weight for the working sets still to do. */
export interface Progression {
	step_kg: number;
	/** Recommended: every working set reached the top of its rep range last time. */
	suggested: boolean;
	/** The sets to change: the new weight, and reps back at the bottom of the range. */
	sets: { id: string; weight_kg: number; reps: number }[];
}

/**
 * What adding one step of weight would change: each working set still to do
 * gets the step on top of its weight. Null when no set is left to change or
 * the equipment has no step.
 *
 * It is suggested when every working set of the last session had a rep range
 * and reached its top, until any working set's weight is no longer last
 * time's: the weights were then chosen by hand, or the step was already added.
 */
export function progression(equipment: Equipment, last: SetView[], current: SetView[]): Progression | null {
	const step = WEIGHT_STEP_KG[equipment];
	if (!step) return null;
	const working = current.filter((s) => !s.warmup);
	const sets = working
		.filter((s) => !s.done)
		.map((s) => ({
			id: s.id,
			weight_kg: Math.round((s.weight_kg + step) * 100) / 100,
			reps: s.rep_max > 0 ? s.rep_min : s.reps
		}));
	if (sets.length === 0) return null;
	const done = last.filter((s) => !s.warmup);
	const suggested =
		done.length > 0 &&
		done.every((s) => s.rep_max > 0 && s.reps >= s.rep_max) &&
		working.every((s, n) => s.weight_kg === (done[n] ?? done[done.length - 1]).weight_kg);
	return { step_kg: step, suggested, sets };
}
