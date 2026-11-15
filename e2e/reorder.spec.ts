import { addExercise, exerciseMenu, exerciseOrder, expect, newWorkout, test, uid } from './helpers';

test('reorder exercises and build supersets', async ({ page }) => {
	const id = uid();
	const [a, b, c] = [`A ${id}`, `B ${id}`, `C ${id}`];
	await newWorkout(page, `Order ${id}`);
	for (const n of [a, b, c]) await addExercise(page, n);

	// Single exercise moves, from the exercise's options menu.
	await exerciseMenu(page, c, `Move ${c} up`);
	await expect.poll(() => exerciseOrder(page)).toEqual([a, c, b]);

	// Superset c + b, then move the superset to the top.
	await exerciseMenu(page, b, 'Superset with above');
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();
	await exerciseMenu(page, b, 'Move superset up');
	await expect.poll(() => exerciseOrder(page)).toEqual([c, b, a]);
	// Rest only after the last exercise of the superset round.
	await expect(page.getByRole('region', { name: c }).getByLabel(/^Rest after/)).toHaveCount(0);
	await expect(page.getByRole('region', { name: b }).getByLabel(/^Rest after/)).toHaveCount(1);

	// Reorder inside the superset.
	await exerciseMenu(page, b, `Move ${b} up in superset`);
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);

	// Survives a reload.
	await page.reload();
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();

	// Saving as a routine keeps the superset.
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await page.getByRole('button', { name: 'Save as routine' }).click();
	await expect(page).toHaveURL(/\/routines\//);
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();

	// Unlinking dissolves a two-exercise superset.
	await exerciseMenu(page, c, 'Remove from superset');
	await expect(page.getByText('Superset', { exact: true })).toHaveCount(0);
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
});
