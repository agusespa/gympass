import { MAX_DURATION_SECONDS, MAX_REPS, MAX_WEIGHT_KG } from './limits';

/** 82.5 → "82.5", 80 → "80". */
export function kg(n: number): string {
	return String(Math.round(n * 100) / 100);
}

/** Parse a weight the iPhone keyboard may type with a decimal comma. */
export function parseWeight(s: string): number {
	const n = parseFloat(s.replace(',', '.'));
	return Number.isFinite(n) && n >= 0 ? Math.min(MAX_WEIGHT_KG, Math.round(n * 100) / 100) : 0;
}

export function parseReps(s: string): number {
	const n = parseInt(s, 10);
	return Number.isFinite(n) && n >= 0 ? Math.min(MAX_REPS, n) : 0;
}

/** A typed set length in seconds: "45" → 45, "1:30" → 90, "1:02:05" → 3725. 0 when unreadable. */
export function parseDuration(s: string): number {
	const parts = s.trim().replace(/[.,]/g, ':').split(':');
	if (parts.some((p) => !/^\d+$/.test(p)) || parts.length > 3) return 0;
	const n = parts.reduce((total, p) => total * 60 + parseInt(p, 10), 0);
	return Math.min(MAX_DURATION_SECONDS, n);
}

/** A set length, e.g. 45 → "0:45", 90 → "1:30", 3725 → "1:02:05". */
export function durationLabel(seconds: number): string {
	return elapsed(seconds * 1000);
}

/** A set's effort: "0:45" for timed sets, "8" for reps; with the weight when there is one. */
export function setLabel(
	s: { reps: number; weight_kg: number; duration_seconds: number },
	timed: boolean
): string {
	const effort = timed ? durationLabel(s.duration_seconds) : String(s.reps);
	return s.weight_kg ? `${kg(s.weight_kg)} kg × ${effort}` : timed ? effort : `${s.reps} reps`;
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

/** Stopwatch time, e.g. 75_000 → "1:15", 3_725_000 → "1:02:05". */
export function elapsed(ms: number): string {
	const total = Math.max(0, Math.floor(ms / 1000));
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = String(total % 60).padStart(2, '0');
	return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
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

/** Total weight lifted, e.g. 8450 → "8,450 kg", 124_300 → "124 t" (separators follow the locale). */
export function tonnage(kgTotal: number): string {
	if (kgTotal >= 100_000) return `${Math.round(kgTotal / 1000).toLocaleString()} t`;
	if (kgTotal >= 10_000)
		return `${(kgTotal / 1000).toLocaleString(undefined, { maximumFractionDigits: 1 })} t`;
	return `${Math.round(kgTotal).toLocaleString()} kg`;
}

/** "2026-11-09" → "9 Nov" in the device locale. */
export function shortDate(iso: string): string {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });
}
