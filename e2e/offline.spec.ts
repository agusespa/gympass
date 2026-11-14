import { expect, test } from '@playwright/test';
import { addExercise, fillSet, openApp, serverData, setText, uid } from './helpers';

test('works offline and syncs when back online', async ({ page, context, request }) => {
	const id = uid();
	await openApp(page);

	await context.setOffline(true);
	// The app shell comes from the service worker cache.
	const res = await page.reload();
	expect(res?.fromServiceWorker()).toBe(true);
	await expect(page.getByRole('heading', { name: 'Workout' })).toBeVisible();

	await page.getByRole('button', { name: 'Start empty workout' }).click();
	await setText(page.getByLabel('Workout name'), `Offline ${id}`);
	await addExercise(page, `Press ${id}`);
	await fillSet(page, `Press ${id}`, 1, '40', '10');

	// Navigating between screens still works offline.
	await page.getByRole('button', { name: 'Back' }).click();
	await page.getByRole('link', { name: 'Profile', exact: true }).click();
	await page.getByRole('link', { name: 'Workout', exact: true }).click();
	await page.getByRole('link', { name: 'Profile', exact: true }).click();
	await expect(page.getByRole('link', { name: new RegExp(`Offline ${id}`) })).toBeVisible();

	// Nothing has reached the server yet.
	let data = await serverData(request);
	expect(data.workouts?.some((w) => w.name === `Offline ${id}`) ?? false).toBe(false);

	await context.setOffline(false);
	await expect
		.poll(async () => {
			data = await serverData(request);
			const w = data.workouts?.find((r) => r.name === `Offline ${id}`);
			const set = data.sets?.find((s) => s.weight_kg === 40 && s.reps === 10);
			return Boolean(w && set);
		})
		.toBe(true);
});

test('pulls changes made elsewhere', async ({ page, request }) => {
	const id = uid();
	const now = Date.now();
	await request.post('/api/sync', {
		data: {
			cursor: 0,
			changes: {
				workouts: [{ id: `w-${id}`, updated_at: now, deleted: false, name: `Remote ${id}`, date: '2026-11-01' }]
			}
		}
	});
	await openApp(page, '/profile');
	await expect(page.getByRole('link', { name: new RegExp(`Remote ${id}`) })).toBeVisible();

	// A newer remote edit wins over the local copy.
	await request.post('/api/sync', {
		data: {
			cursor: 0,
			changes: {
				workouts: [{ id: `w-${id}`, updated_at: now + 1, deleted: true, name: `Remote ${id}`, date: '2026-11-01' }]
			}
		}
	});
	await page.evaluate(() => window.dispatchEvent(new Event('online')));
	await expect(page.getByRole('link', { name: new RegExp(`Remote ${id}`) })).toHaveCount(0);
});
