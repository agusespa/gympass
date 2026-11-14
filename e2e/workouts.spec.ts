import { expect, test } from '@playwright/test';
import { addExercise, exercise, fillSet, newWorkout, serverData, uid } from './helpers';

test('log a workout, see previous values, edit and delete', async ({ page, request }) => {
	const id = uid();
	const bench = `Bench ${id}`;

	await newWorkout(page, `Push ${id}`);
	await addExercise(page, bench);
	await fillSet(page, bench, 1, '80', '8');
	await exercise(page, bench).getByRole('button', { name: '+ Add set' }).click();
	// A new set copies the previous one.
	await expect(exercise(page, bench).getByLabel('Weight set 2')).toHaveValue('80');
	await fillSet(page, bench, 2, '82,5', '6'); // decimal comma from the iPhone keyboard
	await expect(exercise(page, bench).getByLabel('Weight set 2')).toHaveValue('82.5');

	// Shows in history.
	await page.getByRole('link', { name: 'History' }).click();
	const entry = page.getByRole('link', { name: new RegExp(`Push ${id}`) });
	await expect(entry).toContainText(bench);

	// Next workout with the same exercise shows last time's numbers.
	await newWorkout(page, `Push again ${id}`);
	await page.getByLabel('New exercise name').fill(bench.toLowerCase());
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const again = exercise(page, bench.toLowerCase());
	await again.getByRole('button', { name: '+ Add set' }).click();
	await expect(again.getByTestId('previous')).toHaveText(['8 × 80', '6 × 82.5']);

	// Delete a set and the exercise.
	await again.getByRole('button', { name: 'Delete set 2' }).click();
	await expect(again.getByLabel('Reps set 2')).toHaveCount(0);
	await page.getByRole('button', { name: 'Edit order' }).click();
	await again.getByRole('button', { name: 'Delete' }).click();
	// Nothing is deleted until the second tap.
	await again.getByRole('button', { name: 'Cancel' }).click();
	await expect(again).toHaveCount(1);
	await again.getByRole('button', { name: 'Delete' }).click();
	await again.getByRole('button', { name: 'Confirm delete' }).click();
	await expect(again).toHaveCount(0);

	// Delete the whole workout.
	await page.getByRole('button', { name: 'Delete workout' }).click();
	await page.getByRole('button', { name: 'Confirm delete' }).click();
	await expect(page).toHaveURL('/');
	await expect(page.getByRole('link', { name: new RegExp(`Push again ${id}`) })).toHaveCount(0);

	// Everything reached the server, including the delete as a tombstone.
	await expect
		.poll(async () => {
			const data = await serverData(request);
			const again = data.workouts?.find((r) => r.name === `Push again ${id}`);
			const first = data.workouts?.find((r) => r.name === `Push ${id}`);
			const ex = data.workout_exercises?.find((e) => e.workout_id === first?.id);
			const sets = (data.sets ?? [])
				.filter((s) => s.workout_exercise_id === ex?.id && !s.deleted)
				.sort((x, y) => Number(x.position) - Number(y.position))
				.map((s) => [s.weight_kg, s.reps]);
			return { deleted: again?.deleted, sets };
		})
		.toEqual({
			deleted: true,
			sets: [
				[80, 8],
				[82.5, 6]
			]
		});
});

test('rename a workout and change its date', async ({ page }) => {
	const id = uid();
	await newWorkout(page, `Legs ${id}`);
	await page.getByLabel('Workout date').fill('2026-01-02');
	await page.getByLabel('Workout date').blur();
	await page.getByLabel('Workout name').fill(`Legs day ${id}`);
	await page.getByLabel('Workout name').blur();
	await page.reload();
	await expect(page.getByLabel('Workout name')).toHaveValue(`Legs day ${id}`);
	await expect(page.getByLabel('Workout date')).toHaveValue('2026-01-02');
});

test('suggests exercise names used before', async ({ page }) => {
	const id = uid();
	await newWorkout(page, `Pull ${id}`);
	await addExercise(page, `Deadlift ${id}`);
	await newWorkout(page, `Pull 2 ${id}`);
	await expect(page.locator(`datalist#exercise-names option[value="Deadlift ${id}"]`)).toHaveCount(1);
});
