/** Body-part icons a routine can show, by the split it trains. Mirrors RoutineIcons in internal/store/routines.go. */
export const ROUTINE_ICONS = [
	{ id: 'full', label: 'Full body' },
	{ id: 'upper', label: 'Upper' },
	{ id: 'lower', label: 'Lower / Legs' },
	{ id: 'push', label: 'Push' },
	{ id: 'pull', label: 'Pull' },
	{ id: 'chest', label: 'Chest' },
	{ id: 'back', label: 'Back' },
	{ id: 'shoulders', label: 'Shoulders' },
	{ id: 'arms', label: 'Arms' },
	{ id: 'core', label: 'Core' }
] as const;

export type RoutineIcon = (typeof ROUTINE_ICONS)[number]['id'];

export type BodyPart = 'shoulders' | 'chest' | 'core' | 'back' | 'arms' | 'legs';

/** The parts each icon highlights. Ones with `back` draw the figure from behind. */
export const HIGHLIGHTS: Record<RoutineIcon, BodyPart[]> = {
	full: ['shoulders', 'chest', 'core', 'arms', 'legs'],
	upper: ['shoulders', 'chest', 'core', 'arms'],
	lower: ['legs'],
	push: ['shoulders', 'chest', 'arms'],
	pull: ['back', 'arms'],
	chest: ['chest'],
	back: ['back'],
	shoulders: ['shoulders'],
	arms: ['arms'],
	core: ['core']
};
