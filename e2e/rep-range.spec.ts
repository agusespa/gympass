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
	// Going past the top of the range is fine.
	await fillSet(page, bench, 1, '62.5', '15');
	await fillSet(page, bench, 2, '62.5', '9');
	await finishWorkout(page);

	// Change the range afterwards.
	await page.goto('/');
	await page.getByRole('button', { name: `Edit ${name}` }).click();
	await setText(plan.getByLabel('Reps set 1'), '10');
	await setText(plan.getByLabel('Maximum reps for set 1'), '15');

	// The next session continues from exactly the weight and reps done; changing them is up to you.
	// The range under the reps is the routine's new one.
	await page.goto('/');
	await page.getByRole('button', { name: `Start ${name}` }).click();
	ex = exercise(page, bench);
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('62.5');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('15');
	await expect(ex.getByLabel('Reps set 2')).toHaveValue('9');
	await expect(ex.getByTestId('rep-range')).toHaveText(['10–15', '8–12']);
});

test('the max stays as a guide when the reps pass it', async ({ page, request }) => {
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
	// Raising the reps past the max keeps the max: the range is a guide.
	await setText(reps, '15');
	await expect(max).toHaveValue('12');
	await expect(reps).toHaveValue('15');

	await expect
		.poll(async () => {
			const data = await serverData(request);
			const exId = data.routine_exercises?.find((e) => e.name === row)?.id;
			return (data.routine_sets ?? [])
				.filter((s) => s.routine_exercise_id === exId && !s.deleted)
				.map((s) => [s.reps, s.rep_min, s.rep_max]);
		})
		.toEqual([[15, 15, 12]]);

	// Only clearing the max box removes it.
	await setText(max, '');
	await expect(max).toHaveValue('');
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
