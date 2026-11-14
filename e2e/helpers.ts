import { expect, type APIRequestContext, type Page } from '@playwright/test';

/** Unique suffix so tests sharing the server database don't see each other's data. */
export function uid(): string {
	return Math.random().toString(36).slice(2, 8);
}

export type Row = Record<string, unknown> & { id: string; deleted: boolean };

/** Everything the server has, by table. */
export async function serverData(request: APIRequestContext): Promise<Record<string, Row[]>> {
	const res = await request.post('/api/sync', { data: { cursor: 0, changes: {} } });
	expect(res.ok()).toBe(true);
	return (await res.json()).changes;
}

/** Load the app and wait for the service worker to control the page. */
export async function openApp(page: Page, path = '/') {
	await page.goto(path);
	await page.evaluate(async () => {
		await navigator.serviceWorker.ready;
		if (!navigator.serviceWorker.controller) {
			await new Promise((r) => navigator.serviceWorker.addEventListener('controllerchange', r, { once: true }));
		}
	});
}

export async function newWorkout(page: Page, name: string) {
	await page.goto('/');
	await page.getByRole('button', { name: 'New workout' }).click();
	await expect(page).toHaveURL(/\/workouts\//);
	await setText(page.getByLabel('Workout name'), name);
}

export async function setText(input: ReturnType<Page['getByLabel']>, value: string) {
	await input.fill(value);
	await input.blur();
}

export async function addExercise(page: Page, name: string) {
	await page.getByLabel('New exercise name').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(exercise(page, name)).toBeVisible();
}

export function exercise(page: Page, name: string) {
	return page.getByRole('region', { name, exact: true });
}

/** Fill set number n (1-based) of an exercise. */
export async function fillSet(page: Page, ex: string, n: number, kg: string, reps: string) {
	const section = exercise(page, ex);
	await setText(section.getByLabel(`Weight set ${n}`), kg);
	await setText(section.getByLabel(`Reps set ${n}`), reps);
}

/** Exercise names on the page, in display order. */
export async function exerciseOrder(page: Page): Promise<string[]> {
	return page.locator('section[aria-label]').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') ?? ''));
}
