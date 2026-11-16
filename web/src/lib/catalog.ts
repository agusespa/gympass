import entries from '../../../internal/store/catalog.json';

/** What a set records: reps, or duration_seconds (e.g. a dead hang). */
export type Measure = 'reps' | 'time';
export type Equipment = 'barbell' | 'dumbbell' | 'machine' | 'plate' | 'cable' | 'kettlebell' | 'bodyweight';

export interface CatalogExercise {
	/** What workout and routine rows refer to. */
	id: string;
	name: string;
	measure: Measure;
	equipment: Equipment;
}

/** Every exercise there is; the server reads the same file. */
export const CATALOG = entries as CatalogExercise[];

const byId = new Map(CATALOG.map((c) => [c.id, c]));

export function catalogExercise(id: string): CatalogExercise | undefined {
	return byId.get(id);
}
