import { expect, test } from '@playwright/test';
import { newWorkout, serverData, uid } from './helpers';

// page.route can't see requests that pass through a service worker.
test.use({ serviceWorkers: 'block' });

const accessResponses = {
	redirect: { status: 302, headers: { location: 'https://team.cloudflareaccess.com/cdn-cgi/access/login' } },
	'html login page': { status: 200, contentType: 'text/html', body: '<html>Sign in</html>' }
};

for (const [kind, response] of Object.entries(accessResponses)) {
	test(`expired Access session (${kind}) keeps local data and asks to reload`, async ({ page, request }) => {
		const id = uid();
		await page.route('**/api/sync', (route) => route.fulfill(response));

		await newWorkout(page, `Kept ${id}`);
		await expect(page.getByRole('alert')).toContainText('login has expired');
		await page.getByRole('link', { name: 'History' }).click();
		await expect(page.getByRole('link', { name: new RegExp(`Kept ${id}`) })).toBeVisible();

		// Logged in again: reload syncs what was kept.
		await page.unroute('**/api/sync');
		await page.getByRole('button', { name: 'Reload' }).click();
		await expect(page.getByRole('alert')).toHaveCount(0);
		await expect(page.getByRole('link', { name: new RegExp(`Kept ${id}`) })).toBeVisible();
		await expect
			.poll(async () => (await serverData(request)).workouts?.some((w) => w.name === `Kept ${id}`))
			.toBe(true);
	});
}
