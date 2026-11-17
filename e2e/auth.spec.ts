import { addExercise, expect, fillSet, finishWorkout, newWorkout, serverData, test, uid } from './helpers';

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
		await addExercise(page, 'Barbell Row');
		await fillSet(page, 'Barbell Row', 1, '50', '10');
		await finishWorkout(page);
		await expect(page).toHaveURL('/profile');
		await expect(page.getByRole('link', { name: new RegExp(`Kept ${id}`) })).toBeVisible();

		// No banner: the Profile tab and settings gear get a dot, and Settings says why.
		await expect(page.getByRole('alert')).toHaveCount(0);
		await expect(page.getByTestId('sync-badge')).toHaveCount(2);
		await page.getByRole('link', { name: 'Settings' }).click();
		await expect(page.getByTestId('sync-status')).toHaveText('Login expired');

		// Logged in again: reload syncs what was kept.
		await page.unroute('**/api/sync');
		await page.getByRole('button', { name: 'Log in again' }).click();
		await expect(page.getByTestId('sync-status')).toHaveText('Up to date');
		await expect(page.getByTestId('sync-badge')).toHaveCount(0);
		await expect
			.poll(async () => (await serverData(request)).workouts?.some((w) => w.name === `Kept ${id}`))
			.toBe(true);
	});
}
