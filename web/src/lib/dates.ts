/** Calendar dates as YYYY-MM-DD strings in the device's time zone. */

export function isoDate(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** Local midnight of a YYYY-MM-DD date. */
export function parseIso(iso: string): Date {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d);
}

export function today(): string {
	return isoDate(new Date());
}

export function addDays(iso: string, days: number): string {
	const d = parseIso(iso);
	d.setDate(d.getDate() + days);
	return isoDate(d);
}

/** The Monday of the week holding iso. */
export function weekStart(iso: string): string {
	const d = parseIso(iso);
	return addDays(iso, -((d.getDay() + 6) % 7));
}
