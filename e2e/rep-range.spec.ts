import {
	addExercise,
	addSet,
	exercise,
	expect,
	fillSet,
	finishWorkout,
	serverData,
	setText,
	test,
	uid
} from './helpers';

test('a rep range in a routine says when to add weight', async ({ page, request }) => {
	const id = uid();
	const bench = `Bench ${id}`;
	const name = `Push ${id}`;

	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), name);
	await addExercise(page, bench);
	const plan = exercise(page, bench);
	await setText(plan.getByLabel('Weight set 1'), '60');
	// The decimal keypad's separator types the range.
	await setText(plan.getByLabel('Reps set 1'), '8.12');
	await expect(plan.getByLabel('Reps set 1')).toHaveValue('8–12');
	await addSet(page, bench);
	await expect(plan.getByLabel('Reps set 2')).toHaveValue('8–12');

	await expect
		.poll(async () => {
			const data = await serverData(request);
			const exId = data.routine_exercises?.find((e) => e.name === bench)?.id;
			return (data.routine_sets ?? [])
				.filter((s) => s.routine_exercise_id === exId && !s.deleted)
				.map((s) => [s.reps, s.rep_min, s.rep_max]);
		})
		.toEqual([
			[8, 8, 12],
			[8, 8, 12]
		]);

	// First session: start at the bottom of the range, then reach the top on every set.
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	let ex = exercise(page, bench);
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('8');
	await expect(ex.getByTestId('rep-range')).toHaveText(['8–12', '8–12']);
	await expect(ex.getByTestId('add-weight')).toHaveCount(0);
	await fillSet(page, bench, 1, '60', '12');
	await fillSet(page, bench, 2, '60', '12');
	await finishWorkout(page);

	// Next session keeps the weight, restarts reps at the bottom and says to add weight.
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	ex = exercise(page, bench);
	await expect(ex.getByTestId('add-weight')).toBeVisible();
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('60');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('8');

	// Missing the top on one set means staying at this weight, starting from the reps done.
	await fillSet(page, bench, 1, '62.5', '10');
	await fillSet(page, bench, 2, '62.5', '9');
	await finishWorkout(page);
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	ex = exercise(page, bench);
	await expect(ex.getByTestId('add-weight')).toHaveCount(0);
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('62.5');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('10');
	await expect(ex.getByLabel('Reps set 2')).toHaveValue('9');
});
