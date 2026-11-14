/** 82.5 → "82.5", 80 → "80". */
export function kg(n: number): string {
	return String(Math.round(n * 100) / 100);
}

/** Parse a weight the iPhone keyboard may type with a decimal comma. */
export function parseWeight(s: string): number {
	const n = parseFloat(s.replace(',', '.'));
	return Number.isFinite(n) && n >= 0 ? Math.round(n * 100) / 100 : 0;
}

export function parseReps(s: string): number {
	const n = parseInt(s, 10);
	return Number.isFinite(n) && n >= 0 ? n : 0;
}

/** "2026-11-14" → "Sat 14 Nov 2026" in the device locale. */
export function longDate(iso: string): string {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d).toLocaleDateString(undefined, {
		weekday: 'short',
		day: 'numeric',
		month: 'short',
		year: 'numeric'
	});
}

/** Rest duration as m:ss, e.g. 90 → "1:30". */
export function restLabel(seconds: number): string {
	const m = Math.floor(seconds / 60);
	const s = seconds % 60;
	return `${m}:${String(s).padStart(2, '0')}`;
}

/** Rest choices for the picker: 15 s steps to 5 min, then 30 s steps to 10 min. */
export const REST_CHOICES: number[] = [
	...Array.from({ length: 21 }, (_, i) => i * 15),
	...Array.from({ length: 10 }, (_, i) => 330 + i * 30)
];
