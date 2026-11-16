import {
	addExercise,
	addSet,
	entryOf,
	exercise,
	exerciseMenu,
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
	const bench = 'Bench Press';
	const name = `Push ${id}`;

	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), name);
	await addExercise(page, bench);
	const plan = exercise(page, bench);
	// Reps and a max above them plan a range.
	await setText(plan.getByLabel('Reps set 1'), '8');
	await setText(plan.getByLabel('Maximum reps for set 1'), '12');
	await addSet(page, bench);
	await expect(plan.getByLabel('Reps set 2')).toHaveValue('8');
	await expect(plan.getByLabel('Maximum reps for set 2')).toHaveValue('12');

	await expect
		.poll(async () => {
			const data = await serverData(request);
			const exId = entryOf(data, 'routine_exercises', bench)?.id;
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
	const row = 'Barbell Row';

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
			const exId = entryOf(data, 'routine_exercises', row)?.id;
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
			const exId = entryOf(data, 'routine_exercises', row)?.id;
			return (data.routine_sets ?? [])
				.filter((s) => s.routine_exercise_id === exId && !s.deleted)
				.map((s) => [s.reps, s.rep_min, s.rep_max]);
		})
		.toEqual([[15, 0, 0]]);
});

test('a session at the top of the rep range suggests more weight next time', async ({ page }) => {
	const id = uid();
	const press = 'Dumbbell Shoulder Press';
	const name = `Shoulders ${id}`;

	await page.goto('/');
	await page.getByRole('button', { name: 'New routine' }).click();
	await setText(page.getByLabel('Routine name'), name);
	await addExercise(page, press);
	const plan = exercise(page, press);
	await setText(plan.getByLabel('Reps set 1'), '8');
	await setText(plan.getByLabel('Maximum reps for set 1'), '12');
	await addSet(page, press);
	await expect(plan.getByLabel('Maximum reps for set 2')).toHaveValue('12');
	const start = async () => {
		await page.goto('/');
		await page.getByRole('button', { name: `Start ${name}` }).click();
		await expect(exercise(page, press).getByLabel('Weight set 1')).toBeVisible();
	};
	// A dot on the exercise's ⋯ button says its menu has a suggestion.
	const suggestion = exercise(page, press).getByTestId('progression-badge');

	// One set short of the top: nothing to suggest next time.
	await start();
	await fillSet(page, press, 1, '20', '12');
	await fillSet(page, press, 2, '20', '11');
	await finishWorkout(page);
	await start();
	await expect(exercise(page, press).getByLabel('Reps set 2')).toHaveValue('11');
	await expect(suggestion).toHaveCount(0);
	// The menu still adds a step on request.
	await exerciseMenu(page, press, 'Add 2 kg');
	await expect(exercise(page, press).getByLabel('Weight set 1')).toHaveValue('22');
	await expect(exercise(page, press).getByLabel('Reps set 1')).toHaveValue('8');

	// Every set at the top or past it.
	await fillSet(page, press, 1, '20', '12');
	await fillSet(page, press, 2, '20', '13');
	await finishWorkout(page);
	await start();
	const ex = exercise(page, press);
	// Last time's numbers stay until the suggestion is taken.
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('20');
	await expect(suggestion).toBeVisible();
	// Choosing a weight by hand withdraws it; last time's weight brings it back.
	await setText(ex.getByLabel('Weight set 2'), '21');
	await expect(suggestion).toHaveCount(0);
	await setText(ex.getByLabel('Weight set 2'), '20');
	await expect(suggestion).toBeVisible();
	await exerciseMenu(page, press, 'Add 2 kg');
	await expect(ex.getByLabel('Weight set 1')).toHaveValue('22');
	await expect(ex.getByLabel('Weight set 2')).toHaveValue('22');
	await expect(ex.getByLabel('Reps set 1')).toHaveValue('8');
	await expect(ex.getByLabel('Reps set 2')).toHaveValue('8');
	await expect(suggestion).toHaveCount(0);
	// Another step goes on top.
	await exerciseMenu(page, press, 'Add 2 kg');
	await expect(ex.getByLabel('Weight set 2')).toHaveValue('24');
});
