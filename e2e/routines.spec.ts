import {
	addExercise,
	exercise,
	expect,
	fillSet,
	finishWorkout,
	newWorkout,
	serverData,
	setText,
	test,
	uid
} from './helpers';

test('save a workout as a routine and start a workout from it', async ({ page }) => {
	const id = uid();
	const squat = `Squat ${id}`;

	await newWorkout(page, `Legs ${id}`);
	await addExercise(page, squat);
	await fillSet(page, squat, 1, '100', '5');
	await finishWorkout(page);
	await page.getByRole('link', { name: new RegExp(`Legs ${id}`) }).click();
	await page.getByRole('button', { name: 'Save as routine' }).click();
	await expect(page).toHaveURL(/\/routines\//);
	await expect(page.getByLabel('Routine name')).toHaveValue(`Legs ${id}`);
	await expect(exercise(page, squat).getByLabel('Weight set 1')).toHaveValue('100');

	// Edit the routine template.
	await setText(exercise(page, squat).getByLabel('Reps set 1'), '3');

	// The editor has no Start button; routines are started from the list.
	await expect(page.getByRole('button', { name: 'Start', exact: true })).toHaveCount(0);
	await page.goto('/');
	await page.getByRole('button', { name: `Start Legs ${id}` }).click();
	await expect(page).toHaveURL(/\/workouts\//);
	const ex = exercise(page, squat);
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('100');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('3');
	// Previous values come from the original workout.
	await expect(ex.getByTestId('previous')).toHaveText(['100kg × 5']);
});

test('create a routine from scratch, start it from the list, delete it', async ({ page }) => {
	const id = uid();
	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), `Arms ${id}`);
	await addExercise(page, `Curl ${id}`);
	await fillSet(page, `Curl ${id}`, 1, '15', '12');

	await page.getByRole('button', { name: 'Back' }).click();
	await page.getByRole('button', { name: `Start Arms ${id}` }).click();
	await expect(page.getByLabel('Workout name')).toHaveValue(`Arms ${id}`);
	await expect(exercise(page, `Curl ${id}`).getByLabel('Reps set 1')).toHaveValue('12');

	// Discarding the workout returns to the start screen.
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await page.getByRole('button', { name: 'Discard workout' }).click();
	await page
		.getByRole('dialog', { name: 'Discard workout?' })
		.getByRole('button', { name: 'Discard', exact: true })
		.click();
	await expect(page).toHaveURL('/');
	const routine = page.getByRole('list').getByRole('link', { name: `Arms ${id}` });
	await routine.click();
	await page.getByRole('button', { name: 'Delete routine' }).click();
	await page
		.getByRole('dialog', { name: 'Delete routine?' })
		.getByRole('button', { name: 'Delete', exact: true })
		.click();
	await expect(page).toHaveURL('/');
	await expect(routine).toHaveCount(0);
});

test('warm-up sets in a routine carry into the workout', async ({ page, request }) => {
	const id = uid();
	const press = `Press ${id}`;
	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), `Warm ${id}`);
	await addExercise(page, press);
	const ex = exercise(page, press);
	await fillSet(page, press, 1, '40', '10');
	await ex.getByRole('button', { name: 'Add set' }).click();
	await fillSet(page, press, 2, '80', '5');

	// Mark the first set as a warm-up; working sets are numbered on their own.
	await ex.getByRole('button', { name: 'Set 1 type: normal' }).click();
	await ex.getByRole('menuitem', { name: 'Warm-up set' }).click();
	await expect(ex.getByRole('button', { name: /^Set \d type/ })).toHaveText(['W', '1']);

	// The last set defaults to no rest, the others to 2:00; the warm-up gets a shorter one.
	await expect(ex.getByLabel('Rest after set 1')).toHaveValue('120');
	await expect(ex.getByLabel('Rest after set 2')).toHaveValue('0');
	await ex.getByLabel('Rest after set 1').selectOption('60');
	await expect(ex.locator('.rest-pill')).toHaveText(['1:00', '0:00']);

	// The flag reaches the server.
	await expect
		.poll(async () => {
			const data = await serverData(request);
			return (
				data.routine_sets?.filter((s) => s.weight_kg === 40 && s.warmup === true && s.rest_seconds === 60)
					.length ?? 0
			);
		})
		.toBeGreaterThan(0);

	await page.getByRole('button', { name: 'Back' }).click();
	await page.getByRole('button', { name: `Start Warm ${id}` }).click();
	await expect(page).toHaveURL(/\/workouts\//);
	const w = exercise(page, press);
	await expect(w.getByRole('button', { name: /^Set \d type/ })).toHaveText(['W', '1']);
	await expect(w.getByLabel('Weight set 1')).toHaveValue('40');
	await expect(w.getByLabel('Rest after set 1')).toHaveValue('60');
	await expect(w.getByLabel('Rest after set 2')).toHaveValue('0');

	// Turning it back into a normal set renumbers.
	await w.getByRole('button', { name: 'Set 1 type: warm-up' }).click();
	await w.getByRole('menuitem', { name: 'Normal set' }).click();
	await expect(w.getByRole('button', { name: /^Set \d type/ })).toHaveText(['1', '2']);
});
