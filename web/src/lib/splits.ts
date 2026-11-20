/** The splits a routine can train. Mirrors RoutineSplits in internal/store/routines.go. */
export const SPLITS = [
	{ id: 'full', label: 'Whole body' },
	{ id: 'upper', label: 'Upper' },
	{ id: 'lower', label: 'Lower' },
	{ id: 'push', label: 'Push' },
	{ id: 'pull', label: 'Pull' },
	{ id: 'custom', label: 'Custom' }
] as const;

export type Split = (typeof SPLITS)[number]['id'];

export type BodyPart = 'shoulders' | 'chest' | 'core' | 'back' | 'arms' | 'legs';

/** The body parts each split's icon lights up; a figure with `back` is drawn from behind. Custom has no figure. */
export const HIGHLIGHTS: Partial<Record<Split, BodyPart[]>> = {
	full: ['shoulders', 'chest', 'core', 'arms', 'legs'],
	upper: ['shoulders', 'chest', 'core', 'arms'],
	lower: ['legs'],
	push: ['shoulders', 'chest', 'arms'],
	pull: ['back', 'arms']
};

/** What to call a routine's split, or '' when it has none. A custom split without a name is just "Custom". */
export function splitLabel(split: Split | '', name: string): string {
	if (split === 'custom') return name || 'Custom';
	return SPLITS.find((s) => s.id === split)?.label ?? '';
}
