import { test as base, expect, type APIRequestContext, type Page } from '@playwright/test';

export { expect };

/**
 * Tests share one server database, and a running workout takes over the start screen
 * of every client that syncs it. So after each test, discard any workout left running.
 */
export const test = base.extend<{ discardRunning: void }>({
	discardRunning: [
		async ({ request }, use) => {
			await use();
			const running = ((await serverData(request)).workouts ?? []).filter(
				(w) => !w.deleted && w.finished_at == null
			);
			if (running.length === 0) return;
			const now = Date.now();
			const res = await request.post('/api/sync', {
				data: {
					cursor: 0,
					changes: { workouts: running.map((w) => ({ ...w, deleted: true, updated_at: now })) }
				}
			});
			expect(res.ok()).toBe(true);
		},
		{ auto: true }
	]
});

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
	await page.getByRole('button', { name: 'Start empty workout' }).click();
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

/** Fill set number n (1-based) of an exercise and, in a workout, check it off. */
export async function fillSet(page: Page, ex: string, n: number, kg: string, reps: string) {
	const section = exercise(page, ex);
	await setText(section.getByLabel(`Weight set ${n}`), kg);
	await setText(section.getByLabel(`Reps set ${n}`), reps);
	const check = section.getByRole('button', { name: `Complete set ${n}`, exact: true });
	if ((await check.count()) > 0 && (await check.getAttribute('aria-pressed')) !== 'true') {
		await check.click();
		// Checking off an exercise's last set in a running workout folds the exercise away.
		await expect(check.and(page.locator('[aria-pressed="true"]')).or(folded(page, ex))).toBeVisible();
	}
}

/** The name button of an exercise folded away because all its sets are done. */
export function folded(page: Page, ex: string) {
	return exercise(page, ex).getByRole('button', { expanded: false }).filter({ hasText: ex });
}

/** Open an exercise folded away after all its sets were checked off; no-op if it is open. */
export async function unfold(page: Page, ex: string) {
	const button = folded(page, ex);
	if ((await button.count()) > 0) await button.click();
}

/** Add n sets to an exercise: the button in a routine, the exercise menu in a workout. */
export async function addSet(page: Page, ex: string, n = 1) {
	const button = exercise(page, ex).getByRole('button', { name: 'Add set', exact: true });
	for (let i = 0; i < n; i++) {
		if ((await button.count()) > 0) await button.click();
		else await exerciseMenu(page, ex, 'Add set');
	}
}

/** Open an exercise's ⋯ menu and pick an item; the menu closes after it. */
export async function exerciseMenu(page: Page, ex: string, item: string) {
	const section = exercise(page, ex);
	await section.getByRole('button', { name: 'Exercise options' }).click();
	await section.getByRole('menuitem', { name: item, exact: true }).click();
	await expect(section.getByRole('menu')).toHaveCount(0);
}

/** Exercise names on the page, in display order. */
export async function exerciseOrder(page: Page): Promise<string[]> {
	return page.locator('section[aria-label]').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') ?? ''));
}

/** Tap Finish on a running workout, then Save on the summary screen. */
export async function finishWorkout(page: Page) {
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await expect(page).toHaveURL(/\/finish$/);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL('/profile');
}
