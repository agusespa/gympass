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

/** Rest choices for the picker: 0 to 5 min in 30 s steps. */
export const REST_CHOICES: number[] = Array.from({ length: 11 }, (_, i) => i * 30);

/** Workout length, e.g. 4_320_000 → "1 h 12 min", 1_500_000 → "25 min". Empty when not positive. */
export function duration(ms: number): string {
	const min = Math.round(ms / 60000);
	if (!(min > 0)) return '';
	const h = Math.floor(min / 60);
	const m = min % 60;
	return h > 0 ? (m > 0 ? `${h} h ${m} min` : `${h} h`) : `${m} min`;
}
