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

test('a rep range in a routine carries into its workouts', async ({ page, request }) => {
	const id = uid();
	const bench = `Bench ${id}`;
	const name = `Push ${id}`;

	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), name);
	await addExercise(page, bench);
	const plan = exercise(page, bench);
	await setText(plan.getByLabel('Weight set 1'), '60');
	// Reps and a max above them plan a range.
	await setText(plan.getByLabel('Reps set 1'), '8');
	await setText(plan.getByLabel('Maximum reps for set 1'), '12');
	await addSet(page, bench);
	await expect(plan.getByLabel('Reps set 2')).toHaveValue('8');
	await expect(plan.getByLabel('Maximum reps for set 2')).toHaveValue('12');

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

	// First session starts at the bottom of the range, with the range under the reps.
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	let ex = exercise(page, bench);
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('8');
	await expect(ex.getByTestId('rep-range')).toHaveText(['8–12', '8–12']);
	await fillSet(page, bench, 1, '62.5', '12');
	await fillSet(page, bench, 2, '62.5', '9');
	await finishWorkout(page);

	// The next session continues from the weight and reps done; changing the weight is up to you.
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	ex = exercise(page, bench);
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('62.5');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('12');
	await expect(ex.getByLabel('Reps set 2')).toHaveValue('9');
});

test('a max at or below the reps leaves a fixed count', async ({ page, request }) => {
	const id = uid();
	const row = `Row ${id}`;

	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), `Pull ${id}`);
	await addExercise(page, row);
	const plan = exercise(page, row);
	const reps = plan.getByLabel('Reps set 1');
	const max = plan.getByLabel('Maximum reps for set 1');

	await setText(reps, '8');
	await setText(max, '12');
	// Raising the reps past the max drops the range.
	await setText(reps, '15');
	await expect(max).toHaveValue('');
	// So does a max that isn't above the reps.
	await setText(max, '10');
	await expect(max).toHaveValue('');
	await expect(reps).toHaveValue('15');

	await expect
		.poll(async () => {
			const data = await serverData(request);
			const exId = data.routine_exercises?.find((e) => e.name === row)?.id;
			return (data.routine_sets ?? [])
				.filter((s) => s.routine_exercise_id === exId && !s.deleted)
				.map((s) => [s.reps, s.rep_min, s.rep_max]);
		})
		.toEqual([[15, 0, 0]]);
});
