import {
	addExercise,
	exercise,
	expect,
	fillSet,
	finishWorkout,
	newWorkout,
	serverData,
	test,
	uid
} from './helpers';

test('log a workout, see previous values, edit and delete', async ({ page, request }) => {
	const id = uid();
	const bench = `Bench ${id}`;

	await newWorkout(page, `Push ${id}`);
	// Focused screen: no tab bar, just the back button.
	await expect(page.getByRole('link', { name: 'Profile', exact: true })).toHaveCount(0);
	await addExercise(page, bench);
	await fillSet(page, bench, 1, '80', '8');
	await exercise(page, bench).getByRole('button', { name: 'Add set' }).click();
	// A new set copies the previous one.
	await expect(exercise(page, bench).getByLabel('Weight set 2')).toHaveValue('80');
	await fillSet(page, bench, 2, '82,5', '6'); // decimal comma from the iPhone keyboard
	await expect(exercise(page, bench).getByLabel('Weight set 2')).toHaveValue('82.5');

	// Finish opens a summary of the session; warm-ups aside, every logged set counts.
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await expect(page).toHaveURL(/\/finish$/);
	await expect(page.getByText('Sets', { exact: true }).locator('..')).toContainText('2');
	await expect(page.getByText('Volume', { exact: true }).locator('..')).toContainText('1135 kg');
	await expect(page.getByRole('list', { name: 'Exercises done' })).toContainText(
		`${bench} 2 sets · best 82.5 kg × 6`
	);

	// Saving goes to history; the workout is no longer in progress.
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL('/profile');
	const entry = page.getByRole('link', { name: new RegExp(`Push ${id}`) });
	await expect(entry).toContainText(bench);
	await expect(entry).not.toContainText('In progress');
	await entry.click();
	await expect(page.getByRole('button', { name: 'Finish', exact: true })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Delete workout' })).toBeVisible();

	// Next workout with the same exercise shows last time's numbers.
	await newWorkout(page, `Push again ${id}`);
	await page.getByLabel('New exercise name').fill(bench.toLowerCase());
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const again = exercise(page, bench.toLowerCase());
	await again.getByRole('button', { name: 'Add set' }).click();
	await expect(again.getByTestId('previous')).toHaveText(['80kg × 8', '82.5kg × 6']);

	// Delete a set and the exercise.
	await again.getByRole('button', { name: 'Delete set 2' }).click();
	await expect(again.getByLabel('Reps set 2')).toHaveCount(0);
	await again.getByRole('button', { name: 'Exercise options' }).click();
	await again.getByRole('button', { name: 'Delete exercise' }).click();
	// Nothing is deleted until the second tap.
	await again.getByRole('button', { name: 'Cancel' }).click();
	await expect(again).toHaveCount(1);
	await again.getByRole('button', { name: 'Delete exercise' }).click();
	await page
		.getByRole('dialog', { name: 'Delete exercise?' })
		.getByRole('button', { name: 'Delete', exact: true })
		.click();
	await expect(again).toHaveCount(0);

	// The running screen has no bottom buttons; discarding is on the Finish screen.
	await expect(page.getByRole('button', { name: 'Discard workout' })).toHaveCount(0);
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await page.getByRole('button', { name: 'Discard workout' }).click();
	const confirmDiscard = page.getByRole('dialog', { name: 'Discard workout?' });
	await expect(confirmDiscard).toBeVisible();
	await confirmDiscard.getByRole('button', { name: 'Discard', exact: true }).click();
	await expect(page).toHaveURL('/');
	await page.getByRole('link', { name: 'Profile', exact: true }).click();
	await expect(page.getByRole('link', { name: new RegExp(`Push ${id}`) })).toBeVisible();
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
			return {
				deleted: again?.deleted,
				finished: typeof first?.finished_at === 'number',
				timed: Number(first?.finished_at) >= Number(first?.started_at),
				sets
			};
		})
		.toEqual({
			deleted: true,
			finished: true,
			timed: true,
			sets: [
				[80, 8],
				[82.5, 6]
			]
		});
});

test('rename a workout and change its date', async ({ page }) => {
	const id = uid();
	await newWorkout(page, `Legs ${id}`);
	// A running workout is today, so there is no date to change yet.
	await expect(page.getByLabel('Workout date')).toHaveCount(0);
	await page.getByLabel('Workout name').fill(`Legs day ${id}`);
	await page.getByLabel('Workout name').blur();
	await addExercise(page, `Squat ${id}`);
	await fillSet(page, `Squat ${id}`, 1, '60', '5');

	// Once finished, the date can be fixed from history.
	await finishWorkout(page);
	await page.getByRole('link', { name: new RegExp(`Legs day ${id}`) }).click();
	await page.getByLabel('Workout date').fill('2026-01-02');
	await page.getByLabel('Workout date').blur();
	await page.reload();
	await expect(page.getByLabel('Workout name')).toHaveValue(`Legs day ${id}`);
	await expect(page.getByLabel('Workout date')).toHaveValue('2026-01-02');
});

test('suggests exercise names used before', async ({ page }) => {
	const id = uid();
	await newWorkout(page, `Pull ${id}`);
	await addExercise(page, `Deadlift ${id}`);
	await fillSet(page, `Deadlift ${id}`, 1, '140', '3');
	await finishWorkout(page);
	await newWorkout(page, `Pull 2 ${id}`);
	await expect(page.locator(`datalist#exercise-names option[value="Deadlift ${id}"]`)).toHaveCount(1);
});

test('a running workout takes over the app until it is finished', async ({ page, request }) => {
	const id = uid();
	await newWorkout(page, `Arms ${id}`);
	await addExercise(page, `Curl ${id}`);
	const workoutUrl = page.url();

	// No back button and no tab bar: it is left by finishing or discarding it.
	await expect(page.getByRole('button', { name: 'Back' })).toHaveCount(0);
	// Reopening the app goes straight back to it.
	await page.goto('/');
	await expect(page).toHaveURL(workoutUrl);
	await expect(page.getByLabel('Workout name')).toHaveValue(`Arms ${id}`);

	// Saving with nothing filled in offers to discard or keep logging.
	await page.getByRole('button', { name: 'Finish', exact: true }).click();
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	const empty = page.getByRole('dialog', { name: 'Nothing logged yet' });
	await expect(empty).toBeVisible();
	await empty.getByRole('button', { name: 'Keep logging' }).click();
	await expect(page).toHaveURL(/\/workouts\/[^/]+$/);
	await fillSet(page, `Curl ${id}`, 1, '12', '10');

	// Saving needs no confirmation; the toast's Undo puts the workout back in progress.
	await finishWorkout(page);
	await expect(page).toHaveURL('/profile');
	await expect(page.getByRole('status')).toContainText('Workout finished');
	const serverFinishedAt = async () => {
		const data = await serverData(request);
		const w = data.workouts?.find((r) => r.name === `Arms ${id}`);
		return w ? w.finished_at : 'missing';
	};
	await expect.poll(serverFinishedAt).toEqual(expect.any(Number));
	await page.getByRole('button', { name: 'Undo' }).click();
	await expect(page).toHaveURL(/\/workouts\//);
	await expect(page.getByRole('button', { name: 'Finish', exact: true })).toBeVisible();
	// The undo reaches the server too, so the webhook won't send a running workout.
	await expect.poll(serverFinishedAt).toBeNull();

	// Once finished, the start screen is back.
	await finishWorkout(page);
	await page.getByRole('link', { name: 'Workout', exact: true }).click();
	await expect(page).toHaveURL('/');
	await expect(page.getByRole('button', { name: 'Start empty workout' })).toBeVisible();
});
