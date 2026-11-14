import { expect, test } from '@playwright/test';
import { addExercise, exerciseOrder, newWorkout, uid } from './helpers';

test('reorder exercises and build supersets', async ({ page }) => {
	const id = uid();
	const [a, b, c] = [`A ${id}`, `B ${id}`, `C ${id}`];
	await newWorkout(page, `Order ${id}`);
	for (const n of [a, b, c]) await addExercise(page, n);

	// Single exercise moves, from the exercise's options menu.
	await page.getByRole('region', { name: c }).getByRole('button', { name: 'Exercise options' }).click();
	await page.getByRole('button', { name: `Move ${c} up` }).click();
	await expect.poll(() => exerciseOrder(page)).toEqual([a, c, b]);

	// Superset c + b, then move the superset to the top.
	await page.getByRole('region', { name: b }).getByRole('button', { name: 'Exercise options' }).click();
	await page.getByRole('region', { name: b }).getByRole('button', { name: 'Superset with above' }).click();
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Move superset up' }).click();
	await expect.poll(() => exerciseOrder(page)).toEqual([c, b, a]);

	// Reorder inside the superset.
	await page.getByRole('button', { name: `Move ${b} up in superset` }).click();
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);

	// Survives a reload.
	await page.reload();
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();

	// Saving as a routine keeps the superset.
	await page.getByRole('button', { name: 'Save as routine' }).click();
	await expect(page).toHaveURL(/\/routines\//);
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
	await expect(page.getByText('Superset', { exact: true })).toBeVisible();

	// Unlinking dissolves a two-exercise superset.
	await page.getByRole('region', { name: c }).getByRole('button', { name: 'Exercise options' }).click();
	await page.getByRole('region', { name: c }).getByRole('button', { name: 'Remove from superset' }).click();
	await expect(page.getByText('Superset', { exact: true })).toHaveCount(0);
	await expect.poll(() => exerciseOrder(page)).toEqual([b, c, a]);
});
