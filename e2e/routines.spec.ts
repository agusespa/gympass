import { expect, test } from '@playwright/test';
import { addExercise, exercise, fillSet, newWorkout, setText, uid } from './helpers';

test('save a workout as a routine and start a workout from it', async ({ page }) => {
	const id = uid();
	const squat = `Squat ${id}`;

	await newWorkout(page, `Legs ${id}`);
	await addExercise(page, squat);
	await fillSet(page, squat, 1, '100', '5');
	await page.getByRole('button', { name: 'Save as routine' }).click();
	await expect(page).toHaveURL(/\/routines\//);
	await expect(page.getByLabel('Routine name')).toHaveValue(`Legs ${id}`);
	await expect(exercise(page, squat).getByLabel('Weight set 1')).toHaveValue('100');

	// Edit the routine template.
	await setText(exercise(page, squat).getByLabel('Reps set 1'), '3');

	// The editor has no Start button; routines are started from the list.
	await expect(page.getByRole('button', { name: 'Start', exact: true })).toHaveCount(0);
	await page.getByRole('link', { name: 'Workout', exact: true }).click();
	await page.getByRole('button', { name: `Start Legs ${id}` }).click();
	await expect(page).toHaveURL(/\/workouts\//);
	const ex = exercise(page, squat);
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('100');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('3');
	// Previous values come from the original workout.
	await expect(ex.getByTestId('previous')).toHaveText(['5 × 100']);
});

test('create a routine from scratch, start it from the list, delete it', async ({ page }) => {
	const id = uid();
	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), `Arms ${id}`);
	await addExercise(page, `Curl ${id}`);
	await fillSet(page, `Curl ${id}`, 1, '15', '12');

	await page.getByRole('link', { name: 'Workout', exact: true }).click();
	await page.getByRole('button', { name: `Start Arms ${id}` }).click();
	await expect(page.getByLabel('Workout name')).toHaveValue(`Arms ${id}`);
	await expect(exercise(page, `Curl ${id}`).getByLabel('Reps set 1')).toHaveValue('12');

	await page.goto('/');
	await page.getByRole('link', { name: `Arms ${id}` }).click();
	await page.getByRole('button', { name: 'Delete routine' }).click();
	await page.getByRole('button', { name: 'Confirm delete' }).click();
	await expect(page).toHaveURL('/');
	await expect(page.getByRole('link', { name: `Arms ${id}` })).toHaveCount(0);
});
